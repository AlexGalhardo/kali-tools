package tools_test

import (
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/tools"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/porttest"
)

func TestToolRunnerContract(t *testing.T) {
	porttest.ToolRunnerContract(t, func(*testing.T) app.ToolRunner { return tools.Runner{} })
}
