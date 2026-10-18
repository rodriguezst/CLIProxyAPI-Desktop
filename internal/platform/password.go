package platform

import (
	"context"
	"errors"

	"cliproxyapi-desktop/internal/bootstrap"
	"github.com/ncruces/zenity"
)

// ManagementKey keeps entered secrets in memory and never in process arguments.
func ManagementKey(ctx context.Context, reset bool) (string, error) {
	title := "Set Management Key"
	text := "Choose the key you will use to sign in to the management UI. Remember it; the app only saves its hash in config.yaml."
	if reset {
		title = "Reset Management Key"
		text = "Choose a new management key. The server will restart if running, which closes active requests."
	}
	options := []zenity.Option{zenity.Title(title), zenity.HideText(), zenity.OKLabel("Continue"), zenity.Context(ctx)}
	for {
		key, err := zenity.Entry(text, options...)
		if errors.Is(err, zenity.ErrCanceled) {
			return "", bootstrap.ErrCanceled
		}
		if err != nil {
			return "", err
		}
		if err := bootstrap.ValidateManagementKey(key); err != nil {
			Alert(err.Error())
			continue
		}
		confirmation, err := zenity.Entry("Enter the management key again to confirm.", options...)
		if errors.Is(err, zenity.ErrCanceled) {
			return "", bootstrap.ErrCanceled
		}
		if err != nil {
			return "", err
		}
		if key != confirmation {
			Alert("The management keys do not match. Please try again.")
			continue
		}
		return key, nil
	}
}
