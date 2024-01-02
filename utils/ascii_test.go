package utils_test

import (
	"strings"
	"testing"

	"captcha-lite/utils"
)

func TestGenerateAscii(t *testing.T) {
	a := utils.GenerateAscii("Teknologi Umum")
	if !strings.Contains(a, "&lt;") {
		t.Error("GenerateAscii should return ascii")
	}
}
