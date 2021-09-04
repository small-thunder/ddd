package models

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/beego/beego/v2/client/httplib"
	"github.com/beego/beego/v2/server/web"
	"gorm.io/gorm"
)

type CodeSignal struct {
	Command []string
	Admin   bool
	Handle  func(sender *Sender) interface{}
}

type Sender struct {
	UserID            int
	ChatID            int
	Type              string
	Contents          []string
	MessageID         int
	Username          string
	IsAdmin           bool
	ReplySenderUserID int
}

func (sender *Sender) Reply(msg string) {
	switch sender.Type {
	case "tg":
		SendTgMsg(sender.UserID, msg)
	case "tgg":
		SendTggMsg(sender.ChatID, sender.UserID, msg, sender.MessageID, sender.Username)
	case "qq":
		SendQQ(int64(sender.UserID), msg)
	case "qqg":
		SendQQGroup(int64(sender.ChatID), int64(sender.UserID), msg)
	}
}

func (sender *Sender) JoinContens() string {
	return strings.Join(sender.Contents, " ")
}

func (sender *Sender) IsQQ() bool {
	return strings.Contains(sender.Type, "qq")
}

func (sender *Sender) IsTG() bool {
	return strings.Contains(sender.Type, "tg")
}

func (sender *Sender) handleJdCookies(handle func(ck *JdCookie)) error {
	cks := GetJdCookies()
	a := sender.JoinContens()
	ok := false
	if !sender.IsAdmin || a == "" {
		for i := range cks {
			if strings.Contains(sender.Type, "qq") {
				if cks[i].QQ == sender.UserID {
					if !ok {
						ok = true
					}
					handle(&cks[i])
				}
			} else if strings.Contains(sender.Type, "tg") {
				if cks[i].Telegram == sender.UserID {
					if !ok {
						ok = true
					}
					handle(&cks[i])
				}
			}
		}
		if !ok {
			sender.Reply("提醒：该账号已过期，请重新登录。" +
				"抓取CK教程点击右边蓝色链接\n\t https://blog.csdn.net/weixin_42400413/article/details/120091786 \n\t \n\t" +
				"——安卓用户须手动提取pt_key和pt_pin，格式如：\"pt_key=xxxxxxxxxx;pt_pin=xxxxxx;\"。\n\t \n\t" +
				"——IOS用户可以使用快捷指令\n\t https://www.icloud.com/shortcuts/f0bf8b56bd0949afabfc00ca6fbfb4c2 \n\t \n\t" +
				"显示“更新账号XXX”/“新增账号XXX”即更新/新增成功!")
			return errors.New("提醒：该账号已过期，请重新登录。\" +\n\t\t\t\t\"抓取CK教程点击右边蓝色链接\\n\\t https://blog.csdn.net/weixin_42400413/article/details/120091786 \\n\\t \\n\\t\" +\n\t\t\t\t\"——安卓用户须手动提取pt_key和pt_pin，格式如：\\\"pt_key=xxxxxxxxxx;pt_pin=xxxxxx;\\\"。\\n\\t \\n\\t\" +\n\t\t\t\t\"——IOS用户可以使用快捷指令\\n\\t https://www.icloud.com/shortcuts/f0bf8b56bd0949afabfc00ca6fbfb4c2 \\n\\t \\n\\t\" +\n\t\t\t\t\"显示“更新账号XXX”/“新增账号XXX”即更新/新增成功!")
		}
	} else {
		cks = LimitJdCookie(cks, a)
		if len(cks) == 0 {
			sender.Reply("没有匹配的账号")
			return errors.New("没有匹配的账号")
		} else {
			for i := range cks {
				handle(&cks[i])
			}
		}
	}
	return nil
}

var codeSignals = []CodeSignal{
	{
		Command: []string{"status", "状态"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			return Count()
		},
	},
	{
		Command: []string{"sign", "打卡", "签到"},
		Handle: func(sender *Sender) interface{} {
			//if sender.Type == "tgg" {
			//	sender.Type = "tg"
			//}
			//if sender.Type == "qqg" {
			//	sender.Type = "qq"
			//}
			zero, _ := time.ParseInLocation("2006-01-02", time.Now().Local().Format("2006-01-02"), time.Local)
			var u User
			var ntime = time.Now()
			var first = false
			total := []int{}
			err := db.Where("number = ?", sender.UserID).First(&u).Error
			if err != nil {
				first = true
				u = User{
					Class:    sender.Type,
					Number:   sender.UserID,
					Coin:     1,
					ActiveAt: ntime,
				}
				if err := db.Create(&u).Error; err != nil {
					return err.Error()
				}
			} else {
				if zero.Unix() > u.ActiveAt.Unix() {
					first = true
				} else {
					return fmt.Sprintf("你打过卡了，许愿币余额%d。", u.Coin)
				}
			}
			if first {
				db.Model(User{}).Select("count(id) as total").Where("active_at > ?", zero).Pluck("total", &total)
				coin := 1
				if total[0]%3 == 0 {
					coin = 2
				}
				if total[0]%13 == 0 {
					coin = 8
				}
				db.Model(&u).Updates(map[string]interface{}{
					"active_at": ntime,
					"coin":      gorm.Expr(fmt.Sprintf("coin+%d", coin)),
				})
				u.Coin += coin
				sender.Reply(fmt.Sprintf("你是打卡第%d人，奖励%d个许愿币，许愿币余额%d。", total[0]+1, coin, u.Coin))
				ReturnCoin(sender)
				return ""
			}
			return nil
		},
	},
	{
		Command: []string{"coin", "许愿币", "余额", "yu", "yue"},
		Handle: func(sender *Sender) interface{} {
			return fmt.Sprintf("余额%d", GetCoin(sender.UserID))
		},
	},
	{
		Command: []string{"qrcode", "扫码", "二维码", "scan"},
		Handle: func(sender *Sender) interface{} {
			url := fmt.Sprintf("http://127.0.0.1:%d/api/login/qrcode.png?tp=%s&uid=%d&gid=%d", web.BConfig.Listen.HTTPPort, sender.Type, sender.UserID, sender.ChatID)
			if sender.Type == "tgg" {
				url += fmt.Sprintf("&mid=%v&unm=%v", sender.MessageID, sender.Username)
			}
			rsp, err := httplib.Get(url).Response()
			if err != nil {
				return nil
			}
			return rsp
		},
	},
	{
		Command: []string{"升级", "更新", "update", "upgrade"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			if err := Update(sender); err != nil {
				return err.Error()
			}
			sender.Reply("小滴滴重启程序")
			Daemon()
			return nil
		},
	},
	{
		Command: []string{"重启", "reload", "restart", "reboot"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			sender.Reply("小滴滴重启程序")
			Daemon()
			return nil
		},
	},
	{
		Command: []string{"get-ua", "ua"},
		Handle: func(sender *Sender) interface{} {
			if !sender.IsAdmin {
				coin := GetCoin(sender.UserID)
				if coin < 0 {
					return "许愿币不足以查看UserAgent。"
				}
				sender.Reply("查看一次扣1个许愿币。")
				RemCoin(sender.UserID, 1)
			}
			return ua
		},
	},
	{
		Command: []string{"set-ua"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			ctt := sender.JoinContens()
			db.Create(&UserAgent{Content: ctt})
			ua = ctt
			return "已更新User-Agent。"
		},
	},
	{
		Command: []string{"任务列表"},
		Admin:   true,
		Handle: func(_ *Sender) interface{} {
			rt := ""
			for i := range Config.Repos {
				for j := range Config.Repos[i].Task {
					rt += fmt.Sprintf("%s\t%s\n", Config.Repos[i].Task[j].Title, Config.Repos[i].Task[j].Cron)
				}
			}
			return rt
		},
	},
	{
		Command: []string{"查询", "query"},
		Handle: func(sender *Sender) interface{} {
			sender.handleJdCookies(func(ck *JdCookie) {
				sender.Reply(ck.Query())
			})
			return nil
		},
	},
	{
		Command: []string{"发送", "通知", "notify", "send"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			if len(sender.Contents) < 2 {
				sender.Reply("发送指令格式错误")
			} else {
				rt := strings.Join(sender.Contents[1:], " ")
				sender.Contents = sender.Contents[0:1]
				if sender.handleJdCookies(func(ck *JdCookie) {
					ck.Push(rt)
				}) == nil {
					return "操作成功"
				}
			}
			return nil
		},
	},
	{
		Command: []string{"翻翻乐"},
		Handle: func(sender *Sender) interface{} {
			cost := Int(sender.JoinContens())
			if cost <= 0 || cost > 10000 {
				cost = 1
			}
			u := &User{}
			if err := db.Where("number = ?", sender.UserID).First(u).Error; err != nil || u.Coin < cost {
				return "许愿币不足，先去打卡吧。"
			}
			baga := 0
			if u.Coin > 100000 {
				baga = u.Coin
				cost = u.Coin
			}
			r := time.Now().Nanosecond() % 10
			if r < 5 || baga > 0 {
				sender.Reply(fmt.Sprintf("很遗憾你失去了%d枚许愿币。", cost))
				cost = -cost
			} else {
				if r == 9 {
					cost *= 2
					sender.Reply(fmt.Sprintf("恭喜你幸运暴击获得%d枚许愿币，20秒后自动转入余额。", cost))
					time.Sleep(time.Second * 20)
				} else {
					sender.Reply(fmt.Sprintf("很幸运你获得%d枚许愿币，10秒后自动转入余额。", cost))
					time.Sleep(time.Second * 10)
				}
				sender.Reply(fmt.Sprintf("%d枚许愿币已到账。", cost))
			}
			db.Model(u).Update("coin", gorm.Expr(fmt.Sprintf("coin + %d", cost)))
			return nil
		},
	},
	{
		Command: []string{"许愿", "愿望", "wish", "hope", "want"},
		Handle: func(sender *Sender) interface{} {
			ct := sender.JoinContens()
			if ct == "" {
				rt := []string{}
				ws := []Wish{}
				tb := db
				if !sender.IsAdmin {
					tb = tb.Where("user_number", sender.UserID)
				} else {
					tb = tb.Where("status != 1")
				}
				tb.Order("id asc").Find(&ws)
				if len(ws) == 0 {
					return "请对我说 许愿 巴拉巴拉"
				}
				for i, w := range ws {
					status := "未达成"
					if w.Status == 1 {
						status = "已撤销"
					} else if w.Status == 2 {
						status = "已达成"
					}
					id := i + 1
					if sender.IsAdmin {
						id = w.ID
					}
					rt = append(rt, fmt.Sprintf("%d. %s [%s]", id, w.Content, status))
				}
				return strings.Join(rt, "\n")
			}
			cost := 88
			if sender.IsAdmin {
				cost = 1
			}
			tx := db.Begin()
			u := &User{}
			if err := tx.Where("number = ?", sender.UserID).First(u).Error; err != nil {
				tx.Rollback()
				return "许愿币不足，先去打卡吧。"
			}
			w := &Wish{
				Content:    ct,
				Coin:       cost,
				UserNumber: sender.UserID,
			}
			if u.Coin < cost {
				tx.Rollback()
				return fmt.Sprintf("许愿币不足，需要%d个许愿币。", cost)
			}
			if err := tx.Create(w).Error; err != nil {
				tx.Rollback()
				return err.Error()
			}
			if tx.Model(u).Update("coin", gorm.Expr(fmt.Sprintf("coin - %d", cost))).RowsAffected == 0 {
				tx.Rollback()
				return "扣款失败"
			}
			tx.Commit()
			(&JdCookie{}).Push(fmt.Sprintf("有人许愿%s，愿望id为%d。", w.Content, w.ID))
			return fmt.Sprintf("收到愿望，已扣除%d个许愿币。", cost)
		},
	},
	{
		Command: []string{"愿望达成", "达成愿望"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			w := &Wish{}
			id := Int(sender.JoinContens())
			if id == 0 {
				return "目标未指定"
			}
			if db.First(w, id).Error != nil {
				return "目标不存在"
			}
			if w.Status == 1 {
				return "愿望已撤销"
			}
			if w.Status == 2 {
				return "愿望已达成"
			}
			if db.Model(w).Update("status", 2).RowsAffected == 0 {
				return "操作失败"
			}
			sender.Reply(fmt.Sprintf("达成了愿望 %s", w.Content))
			return nil
		},
	},
	{
		Command: []string{"run", "执行", "运行"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			name := sender.Contents[0]
			pins := ""
			if len(sender.Contents) > 1 {
				sender.Contents = sender.Contents[1:]
				err := sender.handleJdCookies(func(ck *JdCookie) {
					pins += "&" + ck.PtPin
				})
				if err != nil {
					return nil
				}
			}
			envs := []Env{}
			if pins != "" {
				envs = append(envs, Env{
					Name:  "pins",
					Value: pins,
				})
			}
			runTask(&Task{Path: name, Envs: envs}, sender)
			return nil
		},
	},
	{
		Command: []string{"优先级", "priority"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			priority := Int(sender.Contents[0])
			if len(sender.Contents) > 1 {
				sender.Contents = sender.Contents[1:]
				sender.handleJdCookies(func(ck *JdCookie) {
					ck.Update(Priority, priority)
					sender.Reply(fmt.Sprintf("已设置账号%s(%s)的优先级为%d。", ck.PtPin, ck.Nickname, priority))
				})
			}
			return nil
		},
	},
	{
		Command: []string{"cmd", "command", "命令"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			ct := sender.JoinContens()
			if regexp.MustCompile(`rm\s+-rf`).FindString(ct) != "" {
				return "over"
			}
			cmd(ct, sender)
			return nil
		},
	},
	{
		Command: []string{"环境变量", "environments", "envs"},
		Admin:   true,
		Handle: func(_ *Sender) interface{} {
			rt := []string{}
			envs := GetEnvs()
			if len(envs) == 0 {
				return "未设置任何环境变量"
			}
			for _, env := range envs {
				rt = append(rt, fmt.Sprintf(`%s="%s"`, env.Name, env.Value))
			}
			return strings.Join(rt, "\n")
		},
	},
	{
		Command: []string{"get-env", "env", "e"},
		Handle: func(sender *Sender) interface{} {
			ct := sender.JoinContens()
			if ct == "" {
				return "未指定变量名"
			}
			value := GetEnv(ct)
			if value == "" {
				return "未设置环境变量"
			}
			return fmt.Sprintf("环境变量的值为：" + value)
		},
	},
	{
		Command: []string{"set-env", "se", "export"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			env := &Env{}
			if len(sender.Contents) >= 2 {
				env.Name = sender.Contents[0]
				env.Value = strings.Join(sender.Contents[1:], " ")
			} else if len(sender.Contents) == 1 {
				ss := regexp.MustCompile(`^([^'"=]+)=['"]?([^=]+?)['"]?$`).FindStringSubmatch(sender.Contents[0])
				if len(ss) != 3 {
					return "无法解析"
				}
				env.Name = ss[1]
				env.Value = ss[2]
			} else {
				return "???"
			}
			ExportEnv(env)
			return "操作成功"
		},
	},
	{
		Command: []string{"unset-env", "ue", "unexport", "de"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			UnExportEnv(&Env{
				Name: sender.JoinContens(),
			})
			return "操作成功"
		},
	},
	{
		Command: []string{"降级"},
		Handle: func(sender *Sender) interface{} {
			return "滚"
		},
	},
	{
		Command: []string{"。。。"},
		Handle: func(sender *Sender) interface{} {
			return "你很无语吗？"
		},
	},
	{
		Command: []string{"祈祷", "祈愿", "祈福"},
		Handle: func(sender *Sender) interface{} {
			if _, ok := mx[sender.UserID]; ok {
				return "你祈祷过啦，等下次我忘记了再来吧。"
			}
			mx[sender.UserID] = true
			if db.Model(User{}).Where("number = ? ", sender.UserID).Update(
				"coin", gorm.Expr(fmt.Sprintf("coin + %d", 1)),
			).RowsAffected == 0 {
				return "先去打卡吧你。"
			}
			return "许愿币+1"
		},
	},
	{
		Command: []string{"撤销愿望"},
		Handle: func(sender *Sender) interface{} {
			ReturnCoin(sender)
			return nil
		},
	},
	{
		Command: []string{"reply", "回复"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			if len(sender.Contents) >= 2 {
				replies[sender.Contents[0]] = strings.Join(sender.Contents[1:], " ")
			} else {
				return "操作失败"
			}
			return "操作成功"
		},
	},
	{
		Command: []string{"help", "助力"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			sender.handleJdCookies(func(ck *JdCookie) {
				ck.Update(Help, True)
				sender.Reply(fmt.Sprintf("已设置助力账号%s(%s)", ck.PtPin, ck.Nickname))
			})
			return nil
		},
	},
	{
		Command: []string{"tool", "工具人", "unhelp", "取消助力"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			sender.handleJdCookies(func(ck *JdCookie) {
				ck.Update(Help, False)
				sender.Reply(fmt.Sprintf("已设置取消助力账号%s(%s)", ck.PtPin, ck.Nickname))
			})
			return nil
		},
	},
	{
		Command: []string{"屏蔽", "hack"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			sender.handleJdCookies(func(ck *JdCookie) {
				ck.Update(Hack, True)
				sender.Reply(fmt.Sprintf("已设置屏蔽助力账号%s(%s)", ck.PtPin, ck.Nickname))
			})
			return nil
		},
	},
	{
		Command: []string{"取消屏蔽", "unhack"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			sender.handleJdCookies(func(ck *JdCookie) {
				ck.Update(Hack, False)
				sender.Reply(fmt.Sprintf("已设置取消屏蔽助力账号%s(%s)", ck.PtPin, ck.Nickname))
			})
			return nil
		},
	},
	{
		Command: []string{"转账"},
		Handle: func(sender *Sender) interface{} {
			cost := 1
			if sender.ReplySenderUserID == 0 {
				return "没有转账目标。"
			}
			amount := Int(sender.JoinContens())
			if !sender.IsAdmin {
				if amount <= 0 {
					return "转账金额必须大于等于1。"
				}
			}
			if sender.UserID == sender.ReplySenderUserID {
				db.Model(User{}).Where("number = ?", sender.UserID).Updates(map[string]interface{}{
					"coin": gorm.Expr(fmt.Sprintf("coin - %d", cost)),
				})
				return fmt.Sprintf("转账成功，扣除手续费%d枚许愿币。", cost)
			}
			if amount > 10000 {
				return "单笔转账限额10000。"
			}
			tx := db.Begin()
			s := &User{}
			if err := db.Where("number = ?", sender.UserID).First(&s).Error; err != nil {
				tx.Rollback()
				return "你还没有开通钱包功能。"
			}
			if s.Coin < amount {
				tx.Rollback()
				return "余额不足。"
			}
			real := amount
			if !sender.IsAdmin {
				if amount <= cost {
					tx.Rollback()
					return fmt.Sprintf("转账失败，手续费需要%d个许愿币。", cost)
				}
				real = amount - cost
			} else {
				cost = 0
			}
			r := &User{}
			if err := db.Where("number = ?", sender.ReplySenderUserID).First(&r).Error; err != nil {
				tx.Rollback()
				return "他还没有开通钱包功能"
			}
			if tx.Model(User{}).Where("number = ?", sender.UserID).Updates(map[string]interface{}{
				"coin": gorm.Expr(fmt.Sprintf("coin - %d", amount)),
			}).RowsAffected == 0 {
				tx.Rollback()
				return "转账失败"
			}
			if tx.Model(User{}).Where("number = ?", sender.ReplySenderUserID).Updates(map[string]interface{}{
				"coin": gorm.Expr(fmt.Sprintf("coin + %d", real)),
			}).RowsAffected == 0 {
				tx.Rollback()
				return "转账失败"
			}
			tx.Commit()
			return fmt.Sprintf("转账成功，你的余额%d，他的余额%d，手续费%d。", s.Coin-amount, r.Coin+real, cost)
		},
	},
	{
		Command: []string{"献祭", "导出"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			sender.handleJdCookies(func(ck *JdCookie) {
				sender.Reply(fmt.Sprintf("pt_key=%s;pt_pin=%s;", ck.PtKey, ck.PtPin))
			})
			return nil
		},
	},
}

var mx = map[int]bool{}

func LimitJdCookie(cks []JdCookie, a string) []JdCookie {
	ncks := []JdCookie{}
	if s := strings.Split(a, "-"); len(s) == 2 {
		for i := range cks {
			if i+1 >= Int(s[0]) && i+1 <= Int(s[1]) {
				ncks = append(ncks, cks[i])
			}
		}
	} else if x := regexp.MustCompile(`^[\s\d,]+$`).FindString(a); x != "" {
		xx := regexp.MustCompile(`(\d+)`).FindAllStringSubmatch(a, -1)
		for i := range cks {
			for _, x := range xx {
				if fmt.Sprint(i+1) == x[1] {
					ncks = append(ncks, cks[i])
				}
			}

		}
	} else if a != "" {
		a = strings.Replace(a, " ", "", -1)
		for i := range cks {
			if strings.Contains(cks[i].Note, a) || strings.Contains(cks[i].Nickname, a) || strings.Contains(cks[i].PtPin, a) {
				ncks = append(ncks, cks[i])
			}
		}
	}
	return ncks
}

func ReturnCoin(sender *Sender) {
	tx := db.Begin()
	ws := []Wish{}
	if err := tx.Where("status = 0 and user_number = ?", sender.UserID).Find(&ws).Error; err != nil {
		tx.Rollback()
		sender.Reply(err.Error())
	}
	for _, w := range ws {
		if tx.Model(User{}).Where("number = ? ", sender.UserID).Update(
			"coin", gorm.Expr(fmt.Sprintf("coin + %d", w.Coin)),
		).RowsAffected == 0 {
			tx.Rollback()
			sender.Reply("愿望未达成退还许愿币失败。")
			return
		}
		sender.Reply(fmt.Sprintf("愿望未达成退还%d枚许愿币。", w.Coin))
		if tx.Model(&w).Update(
			"status", 1,
		).RowsAffected == 0 {
			tx.Rollback()
			sender.Reply("愿望未达成退还许愿币失败。")
			return
		}
	}
	tx.Commit()
}
