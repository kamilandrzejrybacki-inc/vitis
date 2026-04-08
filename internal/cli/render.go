package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

func WriteJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func RenderDoctor(w io.Writer, available bool, providerPath string, detail string) error {
	_, err := fmt.Fprintf(w, "provider_available=%t\nprovider_path=%s\ndetail=%s\n", available, providerPath, detail)
	return err
}

func ErrorResult(code model.ErrorCode, message string) *model.RunResult {
	return &model.RunResult{
		Status: model.RunFailed,
		Meta:   model.ResultMeta{},
		Error: &model.RunError{
			Code:    code,
			Message: message,
		},
	}
}
