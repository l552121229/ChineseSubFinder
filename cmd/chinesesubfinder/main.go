package main

import (
	"github.com/allanpk716/ChineseSubFinder/internal"
	"github.com/allanpk716/ChineseSubFinder/internal/dao"
	"github.com/allanpk716/ChineseSubFinder/internal/pkg"
	"github.com/allanpk716/ChineseSubFinder/internal/pkg/hot_fix"
	"github.com/allanpk716/ChineseSubFinder/internal/pkg/log_helper"
	"github.com/allanpk716/ChineseSubFinder/internal/pkg/notify_center"
	"github.com/allanpk716/ChineseSubFinder/internal/types"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

func init() {

	if pkg.OSCheck() == false {
		panic("only support Linux and Windows")
	}

	log = log_helper.GetLogger()
	config = pkg.GetConfig()
}

func main() {
	if log == nil {
		panic("log init error")
	}
	if config == nil {
		panic("read config error")
	}
	httpProxy := config.HttpProxy
	if config.UseProxy == false {
		httpProxy = ""
	}
	// 判断文件夹是否存在
	if pkg.IsDir(config.MovieFolder) == false {
		log.Errorln("MovieFolder not found")
		return
	}
	if pkg.IsDir(config.SeriesFolder) == false {
		log.Errorln("SeriesFolder not found")
		return
	}
	if pkg.IsDir(config.AnimeFolder) == false {
		log.Errorln("AnimeFolder not found")
		return
	}
	// ------ 数据库相关操作 Start ------
	err := dao.InitDb()
	if err != nil {
		log.Errorln("dao.InitDb()", err)
		return
	}
	// ------ 数据库相关操作 End ------

	// ------ Hot Fix Start ------
	// 开始修复
	log.Infoln("HotFix Start...")
	err = hot_fix.HotFixProcess(types.HotFixParam{
		MovieRootDir:  config.MovieFolder,
		SeriesRootDir: config.SeriesFolder,
		AnimeRootDir:  config.AnimeFolder,
	})
	if err != nil {
		log.Errorln("HotFixProcess()", err)
		log.Infoln("HotFix End")
		return
	}
	log.Infoln("HotFix End")
	// ------ Hot Fix End ------

	// 初始化通知缓存模块
	notify_center.Notify = notify_center.NewNotifyCenter(config.WhenSubSupplierInvalidWebHook)

	log.Infoln("MovieFolder:", config.MovieFolder)
	log.Infoln("SeriesFolder:", config.SeriesFolder)
	log.Infoln("AnimeFolder:", config.AnimeFolder)

	// ReloadBrowser 提前把浏览器下载好
	pkg.ReloadBrowser()

	// 任务还没执行完，下一次执行时间到来，下一次执行就跳过不执行
	c := cron.New(cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)))
	// 定时器
	entryID, err := c.AddFunc("@every "+config.EveryTime, func() {

		DownLoadStart(httpProxy)
	})
	if err != nil {
		log.Errorln("cron entryID:", entryID, "Error:", err)
		return
	}
	log.Infoln("First Time Download Start")

	DownLoadStart(httpProxy)

	log.Infoln("First Time Download End")

	c.Start()
	// 阻塞
	select {}
}

func DownLoadStart(httpProxy string) {
	defer func() {
		log.Infoln("Download One End...")
		notify_center.Notify.Send()
		pkg.CloseChrome()
	}()
	notify_center.Notify.Clear()

	// 下载实例
	downloader := internal.NewDownloader(types.ReqParam{
		HttpProxy:                     httpProxy,
		DebugMode:                     config.DebugMode,
		SaveMultiSub:                  config.SaveMultiSub,
		Threads:                       config.Threads,
		SubTypePriority:               config.SubTypePriority,
		WhenSubSupplierInvalidWebHook: config.WhenSubSupplierInvalidWebHook,
		EmbyConfig:                    config.EmbyConfig,
		SaveOneSeasonSub:              config.SaveOneSeasonSub,
	})

	log.Infoln("Download One Started...")

	// 刷新 Emby 的字幕，如果下载了字幕倒是没有刷新，则先刷新一次，便于后续的 Emby api 统计逻辑
	err := downloader.RefreshEmbySubList()
	if err != nil {
		log.Errorln("RefreshEmbySubList", err)
		return
	}
	err = downloader.GetUpdateVideoListFromEmby(config.MovieFolder, config.SeriesFolder, config.AnimeFolder)
	if err != nil {
		log.Errorln("GetUpdateVideoListFromEmby", err)
		return
	}
	// 开始下载，电影
	err = downloader.DownloadSub4Movie(config.MovieFolder)
	if err != nil {
		log.Errorln("DownloadSub4Movie", err)
		return
	}
	// 开始下载，连续剧
	err = downloader.DownloadSub4Series(config.SeriesFolder)
	if err != nil {
		log.Errorln("DownloadSub4Series", err)
		return
	}
	// 开始下载，动漫
	log.Infoln("Download anime start...")
	err = downloader.DownloadSub4Series(config.AnimeFolder)
	if err != nil {
		log.Errorln("DownloadSub4SeriesAnime", err)
		return
	}
	log.Infoln("Download anime end...")
	// 刷新 Emby 的字幕，下载完毕字幕了，就统一刷新一下
	err = downloader.RefreshEmbySubList()
	if err != nil {
		log.Errorln("RefreshEmbySubList", err)
		return
	}
}

var (
	log    *logrus.Logger
	config *types.Config
)
