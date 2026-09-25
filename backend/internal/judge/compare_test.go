package judge

import (
	"testing"

	"akioi/internal/config"
	"akioi/internal/model"
)

func TestCompareDefaultIgnoresTrailingSpace(t *testing.T) {
	expected := []byte("1 2 3\n4 5 6\n")
	actual := []byte("1 2 3  \n4 5 6\n\n\n")
	if got := Compare(model.CompareModeDefault, 1e-6, expected, actual); !got.Accepted {
		t.Fatalf("应忽略行尾空格与末尾空行，实际判定为不通过: %s", got.Message)
	}
}

func TestCompareDefaultDetectsDifferentLine(t *testing.T) {
	expected := []byte("1\n2\n3\n")
	actual := []byte("1\n9\n3\n")
	got := Compare(model.CompareModeDefault, 1e-6, expected, actual)
	if got.Accepted {
		t.Fatal("内容不同应判定为不通过")
	}
	if got.Message != "第 2 行不一致" {
		t.Fatalf("提示信息应只含行号，实际为 %q", got.Message)
	}
}

func TestCompareExactKeepsTrailingSpace(t *testing.T) {
	expected := []byte("abc\n")
	actual := []byte("abc \n")
	if got := Compare(model.CompareModeExact, 1e-6, expected, actual); got.Accepted {
		t.Fatal("严格比对下多余空格应判定为不通过")
	}
}

func TestCompareFloatWithinEpsilon(t *testing.T) {
	expected := []byte("3.1415926 10\n")
	actual := []byte("3.1415927 10.0000001\n")
	if got := Compare(model.CompareModeFloat, 1e-4, expected, actual); !got.Accepted {
		t.Fatalf("误差范围内应判定为通过，实际: %s", got.Message)
	}
	if got := Compare(model.CompareModeFloat, 1e-9, expected, actual); got.Accepted {
		t.Fatal("超出误差范围应判定为不通过")
	}
}

func TestCompareFloatTokenCount(t *testing.T) {
	got := Compare(model.CompareModeFloat, 1e-6, []byte("1 2\n"), []byte("1\n"))
	if got.Accepted {
		t.Fatal("token 数不同应判定为不通过")
	}
}

func TestResolveLanguage(t *testing.T) {
	cfg := &config.JudgeConfig{
		CppCompiler: "g++", CppStd: "c++14", CppFlags: "-O2",
		CCompiler: "gcc", CStd: "c11", CFlags: "-O1",
	}
	cpp, err := ResolveLanguage(model.LangCpp, cfg)
	if err != nil {
		t.Fatalf("解析 C++ 失败: %v", err)
	}
	if cpp.SourceFile() != "/work/main.cpp" {
		t.Fatalf("C++ 源码路径错误: %s", cpp.SourceFile())
	}
	c, err := ResolveLanguage(model.LangC, cfg)
	if err != nil {
		t.Fatalf("解析 C 失败: %v", err)
	}
	if c.SourceFile() != "/work/main.c" {
		t.Fatalf("C 源码路径错误: %s", c.SourceFile())
	}
	if _, err := ResolveLanguage("python", cfg); err == nil {
		t.Fatal("未登记的语言应返回错误")
	}
}
