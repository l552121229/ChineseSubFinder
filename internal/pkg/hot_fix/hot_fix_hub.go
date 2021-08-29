package hot_fix

import (
	"errors"
	"fmt"
	"github.com/allanpk716/ChineseSubFinder/internal/dao"
	"github.com/allanpk716/ChineseSubFinder/internal/ifaces"
	"github.com/allanpk716/ChineseSubFinder/internal/models"
	"github.com/allanpk716/ChineseSubFinder/internal/pkg/log_helper"
	"github.com/allanpk716/ChineseSubFinder/internal/types"
)

// HotFixProcess 去 DB 中查询 Hotfix 的标记，看有那些需要修复，那些已经修复完毕
func HotFixProcess(param types.HotFixParam) error {

	// -----------------------------------------------------------------------
	// 一共有多少个 HotFix 要修复，需要固定下来
	hotfixCases := []ifaces.IHotFix{
		NewHotFix001(param.MovieRootDir, param.SeriesRootDir),
	}
	// -----------------------------------------------------------------------
	// 找现在有多少个 hotfix 执行过了
	var hotFixes []models.HotFix
	result := dao.GetDb().Find(&hotFixes)
	if result.Error != nil {
		return errors.New(fmt.Sprintf("hotfix query all result failed, %s", result.Error))
	}
	// 数据库中是否有记录，记录了是否有运行都需要判断
	var hotFixRecord = make(map[string]models.HotFix)
	for _, fix := range hotFixes {
		hotFixRecord[fix.Key] = fix
	}
	// 交叉对比，这个执行的顺序又上面 []interface.IHotFix 指定
	for _, hotfixCase := range hotfixCases {
		_, bFound := hotFixRecord[hotfixCase.GetKey()]
		if bFound == false {
			// 没有找到那么就需要进行修复
			log_helper.GetLogger().Infoln("hotfix", hotfixCase.GetKey(), "start...")
			processResult, err := hotfixCase.Process()
			// 找到对应的 hotfix 方案进行 interface 数据的转换输出
			switch hotfixCase.GetKey() {
			case "001":
				outStruct := processResult.(OutStruct001)
				if err != nil {
					for i, file := range outStruct.ErrFiles {
						log_helper.GetLogger().Errorln("hotfix 001, rename failed,", i, file)
					}
					// 如果任意故障则跳出后续的修复
					log_helper.GetLogger().Errorln("hotfix 001 failed, break")
					return err
				}
				break
			default:
				continue
			}
			// 执行成功则存入数据库中，标记完成
			markHotFixDone := models.HotFix{Key: hotfixCase.GetKey(), Done: true}
			result = dao.GetDb().Create(&markHotFixDone)
			if result.Error != nil {
				return errors.New(fmt.Sprintf("hotfix %s is done, but record failed, %s", hotfixCase.GetKey(), result.Error))
			}
			log_helper.GetLogger().Infoln("hotfix", hotfixCase.GetKey(), "is done")
		}
		// 找到了，目前的逻辑是成功才插入，那么查询到了，就默认是执行成功了
	}
	return nil

}
