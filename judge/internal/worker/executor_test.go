package worker

import "testing"

func TestContainerConfig(t *testing.T) {
	cases := []struct {
		lang         Language
		wantImage    string
		wantFileName string
	}{
		{LanguagePython, "python:3.11-alpine", "solution.py"},
		{LanguageJavaScript, "node:20-alpine", "solution.js"},
		{LanguageGo, "golang:1.26-alpine", "solution.go"},
		{LanguageSQL, "ascend-sqlite:latest", "solution.sql"},
		{LanguageJava, "eclipse-temurin:21-jdk-alpine", "Solution.java"},
		{LanguageC, "ascend-cpp:latest", "solution.c"},
		{LanguageCPP, "ascend-cpp:latest", "solution.cpp"},
	}

	for _, tt := range cases {
		t.Run(string(tt.lang), func(t *testing.T) {
			image, command, fileName, err := containerConfig(tt.lang)
			if err != nil {
				t.Fatalf("containerConfig(%q): %v", tt.lang, err)
			}
			if image != tt.wantImage {
				t.Errorf("image = %q, want %q", image, tt.wantImage)
			}
			if fileName != tt.wantFileName {
				t.Errorf("fileName = %q, want %q", fileName, tt.wantFileName)
			}
			if len(command) == 0 {
				t.Error("expected a non-empty command")
			}
		})
	}
}

func TestContainerConfig_UnsupportedLanguage(t *testing.T) {
	_, _, _, err := containerConfig(Language("ruby"))
	if err == nil {
		t.Fatal("expected an error for an unsupported language")
	}
}
