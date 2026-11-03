package handler

import (
	"fmt"
	"strings"

	"akioi/internal/model"
)

// resolveSPJ 校验并返回题目的 Special Judge 设置；非 spj 比对方式下清空代码。
func resolveSPJ(compareMode string, language, code *string) (string, string, error) {
	if compareMode != model.CompareModeSPJ {
		return "", "", nil
	}
	lang := model.SPJLanguageCpp
	if language != nil {
		value := strings.TrimSpace(*language)
		if value != "" {
			if value != model.SPJLanguageCpp && value != model.SPJLanguageC {
				return "", "", fmt.Errorf("SPJ 语言只支持 cpp 或 c")
			}
			lang = value
		}
	}
	text := ""
	if code != nil {
		text = strings.TrimSpace(*code)
	}
	if text == "" {
		return "", "", fmt.Errorf("启用 SPJ 时需要填写 SPJ 程序")
	}
	return lang, text, nil
}
