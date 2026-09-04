package launcher

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/local/nblink-companion/internal/config"
	"github.com/local/nblink-companion/internal/model"
)

func Open(rule model.ForwardRule, settings config.Settings) error {
	address := rule.LocalAddress()
	switch rule.Kind {
	case model.ServiceKindWeb:
		scheme := model.NormalizeWebScheme(rule.WebScheme)
		return openURL((&url.URL{Scheme: scheme, Host: address}).String())
	case model.ServiceKindRDP:
		return openRDP(address, settings.RDPLauncher)
	case model.ServiceKindVNC:
		return openVNC(address, settings.VNCLauncher)
	default:
		return errors.New("该服务没有可启动的客户端，请复制固定地址使用")
	}
}

func OpenPath(path string) error {
	return openPath(path)
}

func customLauncherMissing(kind string) error {
	return fmt.Errorf("未配置 %s 客户端，请在设置中选择可执行文件", kind)
}
