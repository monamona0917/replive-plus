package service

import (
	"crypto/tls"
	"fmt"
	"replive/config"
	"replive/dal"
	"replive/model"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/wneessen/go-mail"
	"gopkg.in/gomail.v2"
)

var (
	sendCli      *gomail.Dialer
	mailCli      *mail.Client
	useEmail     bool
	emailChannel chan *EMailInfo
	emailInitMu  sync.Mutex
)

func initEmailSender() {
	emailInitMu.Lock()
	defer emailInitMu.Unlock()
	if emailChannel != nil {
		return
	}
	emailChannel = make(chan *EMailInfo, 1000)
	if len(config.Conf.Email.SmtpHost) == 0 || len(config.Conf.Email.Sender) == 0 || len(config.Conf.Email.AuthCode) == 0 || len(config.Conf.Email.Receiver) == 0 {
		useEmail = false
		hlog.Infof("email config is empty, useEmail: %v", useEmail)
		return
	}
	// 初始化邮件发送器
	sendCli = gomail.NewDialer(config.Conf.Email.SmtpHost, 587, config.Conf.Email.Sender, config.Conf.Email.AuthCode)
	sendCli.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	var err error
	mailCli, err = mail.NewClient(
		config.Conf.Email.SmtpHost,
		mail.WithPort(587),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(config.Conf.Email.Sender),
		mail.WithPassword(config.Conf.Email.AuthCode),
		mail.WithTLSPortPolicy(mail.TLSOpportunistic),
		mail.WithTimeout(time.Minute*10),
	)
	if err != nil {
		hlog.Errorf("Failed to init email client: %v", err)
		panic(err)
	}
	useEmail = true
	hlog.Infof("email sender init done, useEmail: %v", useEmail)
	go asyncSendEmail()
}

type EMailInfo struct {
	Title     string
	Content   string
	FilePath  string
	RetryTime int
	Quiet     bool
}

func enqueueEmail(info *EMailInfo) bool {
	if !useEmail || emailChannel == nil {
		return false
	}
	select {
	case emailChannel <- info:
		return true
	default:
		hlog.Warnf("email channel full, drop email: title=%s retry=%d", info.Title, info.RetryTime)
		return false
	}
}

func sendLiveEmail(liveInfo *model.LiveStream, nsyInfo *model.LiveUser, rtmpUrl string, isFandomOnly bool) {
	content := fmt.Sprintf("标题: %s\nrtmp: %s\n", liveInfo.Title, rtmpUrl)
	if isFandomOnly {
		content += "fandom only 且未加入，无法获取链接\n"
	}
	enqueueEmail(&EMailInfo{
		Title:   fmt.Sprintf("直播开播: %s", nsyInfo.Info.DisplayName),
		Content: content,
		Quiet:   true,
	})
}

func sendLiveEndEmail(nsyName string, fileName string) {
	enqueueEmail(&EMailInfo{
		Title:   fmt.Sprintf("直播结束: %s", nsyName),
		Content: fmt.Sprintf("fileName: %s\n", fileName),
		Quiet:   true,
	})
}

func sendChatEmail(msg *dal.ChatMessage) {
	if !useEmail {
		return
	}
	if msg.MsgType != int32(model.ChatMessageType_Image) && msg.MsgType != int32(model.ChatMessageType_Video) {
		return
	}
	// 12 小时往前的历史的不发，自己去文件夹看就行
	if msg.SendTime < time.Now().Unix()-3600*12 {
		hlog.Infof("don't send email, msg is too old, msgId: %v, sendTime: %v", msg.ChatMessageId, msg.SendTime)
		return
	}

	contextContent, err := buildChatEmailContext(msg)
	if err != nil {
		hlog.Errorf("查询时间相近的几条消息失败: %v", err)
		contextContent = "暂未获取上下文"
	}

	emailInfos := buildMediaEmailInfos(msg, contextContent)
	for _, info := range emailInfos {
		hlog.Infof("enqueue chat email, title: %s, file: %s, msgId: %v", info.Title, info.FilePath, msg.ChatMessageId)
		enqueueEmail(info)
	}
}

func buildChatEmailContext(msg *dal.ChatMessage) (string, error) {
	// 查询前后10分钟内的几条消息，一并发送
	msgList := make([]*dal.ChatMessage, 0)
	err := dal.ReadDB().Table(msg.TableName()).
		Where("chat_room_id = ? AND user_id != ?", msg.ChatRoomId, msg.UserId).
		Where("send_time >= ? AND send_time <= ?", msg.SendTime-600, msg.SendTime+600).
		Order("id").
		Limit(100).
		Find(&msgList).Error
	if err != nil {
		return "", err
	}
	content := ""
	for _, m := range msgList {
		contentLine := m.Content
		if m.MsgType == int32(model.ChatMessageType_Image) {
			contentLine = "图片"
		} else if m.MsgType == int32(model.ChatMessageType_Video) {
			contentLine = "视频"
		}
		content += fmt.Sprintf("%s\n", contentLine)
	}
	if len(content) == 0 {
		content = "暂未获取上下文"
	}
	return content, nil
}

func buildMediaEmailInfos(msg *dal.ChatMessage, contextContent string) []*EMailInfo {
	title, mediaType, remoteURL, filePath := getMediaEmailMeta(msg)
	if title == "" {
		return nil
	}

	summaryContent := fmt.Sprintf(
		"类型: %s\n原始URL: %s\n本地文件: %s\n",
		mediaType, emptyFallback(remoteURL), emptyFallback(filePath),
	)
	summaryContent += "\n上下文:\n" + contextContent

	return []*EMailInfo{
		{
			Title:   title + " [URL]",
			Content: summaryContent,
		},
		{
			Title:    title + " [附件]",
			Content:  fmt.Sprintf("类型: %s\n原始URL: %s\n本地文件: %s\n", mediaType, emptyFallback(remoteURL), emptyFallback(filePath)),
			FilePath: filePath,
		},
	}
}

func getMediaEmailMeta(msg *dal.ChatMessage) (title string, mediaType string, remoteURL string, filePath string) {
	switch msg.MsgType {
	case int32(model.ChatMessageType_Image):
		return fmt.Sprintf("图片消息: %v", msg.DisplayName), "图片", msg.ImageUrl, msg.ImagePath
	case int32(model.ChatMessageType_Video):
		return fmt.Sprintf("视频消息: %v", msg.DisplayName), "视频", msg.VideoUrl, msg.VideoPath
	default:
		return "", "", "", ""
	}
}

func emptyFallback(s string) string {
	if s == "" {
		return "(空)"
	}
	return s
}

func asyncSendEmail() {
	retry := func(info *EMailInfo) {
		info.RetryTime++
		if !enqueueEmail(info) {
			hlog.Warnf("drop retry email: title=%s retry=%d", info.Title, info.RetryTime)
		}
	}
	for info := range emailChannel {
		if info.RetryTime > 10 {
			hlog.Errorf("send email failed, retry time: %v, info: %v", info.RetryTime, info)
			continue
		}
		if info.RetryTime%2 == 0 {
			if err := sendV1(info); err != nil {
				hlog.Errorf("sendV1 email failed, retry time: %v, info: %v, err: %v", info.RetryTime, info, err)
				retry(info)
				continue
			}
		} else {
			if err := sendV2(info); err != nil {
				hlog.Errorf("sendV2 email failed, retry time: %v, info: %v, err: %v", info.RetryTime, info, err)
				retry(info)
				continue
			}
		}
	}
}

func sendV2(info *EMailInfo) error {
	msg := gomail.NewMessage()
	msg.SetHeader("From", config.Conf.Email.Sender)
	msg.SetHeader("To", config.Conf.Email.Receiver)
	msg.SetHeader("Subject", info.Title)
	msg.SetBody("text/plain", info.Content)
	if len(info.FilePath) > 0 {
		msg.Attach(info.FilePath)
	}
	if !info.Quiet {
		hlog.Infof("发送邮件: %v", info)
	}
	if err := sendCli.DialAndSend(msg); err != nil {
		hlog.Errorf("发送邮件失败: %v", err)
		return err
	}
	return nil
}

func sendV1(info *EMailInfo) error {
	message := mail.NewMsg()
	if err := message.From(config.Conf.Email.Sender); err != nil {
		hlog.Errorf("Failed to set from: %v", err)
		return err
	}
	if err := message.To(config.Conf.Email.Receiver); err != nil {
		hlog.Errorf("Failed to set to: %v", err)
		return err
	}
	message.Subject(info.Title)
	message.SetBodyString(mail.TypeTextPlain, info.Content)
	if len(info.FilePath) > 0 {
		message.AttachFile(info.FilePath)
	}
	if !info.Quiet {
		hlog.Infof("发送邮件: %v", info)
	}
	if err := mailCli.DialAndSend(message); err != nil {
		hlog.Errorf("发送邮件失败: %v", err)
		// send next time
		return err
	}
	return nil
}
