package judge

import (
	"fmt"

	"akioi/internal/config"
	"akioi/internal/model"
)

// Language 描述一门受支持语言的编译与运行方式，路径均为沙箱内路径。
type Language struct {
	Key         string
	Name        string
	SourceName  string
	CompileArgs []string
	RunPath     string
}

// SourceFile 返回源码在沙箱工作目录中的文件名。
func (l Language) SourceFile() string { return "/work/" + l.SourceName }

// ResolveLanguage 按语言标识返回编译与运行配置。
func ResolveLanguage(key string, cfg *config.JudgeConfig) (Language, error) {
	switch key {
	case model.LangCpp:
		args := []string{cfg.CppCompiler}
		if cfg.CppStd != "" {
			args = append(args, "-std="+cfg.CppStd)
		}
		args = append(args, splitFlags(cfg.CppFlags)...)
		args = append(args, "-o", "/work/main", "/work/main.cpp")
		return Language{
			Key:         model.LangCpp,
			Name:        "C++",
			SourceName:  "main.cpp",
			CompileArgs: args,
			RunPath:     "/work/main",
		}, nil
	case model.LangC:
		args := []string{cfg.CCompiler}
		if cfg.CStd != "" {
			args = append(args, "-std="+cfg.CStd)
		}
		args = append(args, splitFlags(cfg.CFlags)...)
		args = append(args, "-lm", "-o", "/work/main", "/work/main.c")
		return Language{
			Key:         model.LangC,
			Name:        "C",
			SourceName:  "main.c",
			CompileArgs: args,
			RunPath:     "/work/main",
		}, nil
	}
	return Language{}, fmt.Errorf("不支持的语言: %s", key)
}

func splitFlags(s string) []string {
	var out []string
	field := ""
	for _, r := range s {
		if r == ' ' || r == '\t' {
			if field != "" {
				out = append(out, field)
				field = ""
			}
			continue
		}
		field += string(r)
	}
	if field != "" {
		out = append(out, field)
	}
	return out
}
