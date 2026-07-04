package tui

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/CooDdk/freexclaw/internal/conversation"
	"github.com/CooDdk/freexclaw/internal/tools"
)

const runtimeProfileEngineering = "engineering-delivery"

const builtinEngineeringRuntimePrompt = `当前任务属于代码交付。默认要求：按工程化方式交付，补齐必要目录和依赖，写完后执行会结束的初始化与校验命令，并明确汇报修改文件、执行命令、校验结果和剩余风险。`
const builtinEngineeringSkillSummary = "工程化技能要求：先检查工作区结构，再创建多文件项目骨架；项目生成时不要停留在单个文件或单段代码块，必须使用文件工具逐个落盘，并继续执行依赖初始化、安装和基础校验。"
const builtinDeliveryTemplateSummary = "变更文件 / 执行命令 / 校验结果 / 结果说明 / 剩余风险 / 下一步"

var runtimePromptCache sync.Map

func detectRuntimePromptProfile(userPrompt string, messages []conversation.Message) string {
	lower := strings.ToLower(strings.TrimSpace(userPrompt))
	if lower == "" {
		return ""
	}

	if isCodingRequest(lower) {
		return runtimeProfileEngineering
	}

	if hasRecentCodingContext(messages) && isLikelyCodingFollowUp(lower) {
		return runtimeProfileEngineering
	}

	return ""
}

func isCodingRequest(content string) bool {
	codeHints := []string{
		"代码", "编程", "项目", "服务", "接口", "api", "脚本", "应用", "网站", "前端", "后端", "框架",
		"create", "build", "implement", "scaffold", "project", "service", "api", "script",
		"feature", "backend", "frontend", "refactor", "fix bug", "debug", "write code",
		"go ", "gin", "python", "node", "react", "vue", "typescript", "javascript", "nestjs", "nest", "express",
	}
	actionHints := []string{
		"创建", "生成", "实现", "搭建", "开发", "编写", "写一个", "使用", "修改", "重构", "修复", "优化",
		"测试", "校验", "构建", "运行", "新增", "加一个",
	}
	return containsAny(content, codeHints...) && containsAny(content, actionHints...)
}

func hasRecentCodingContext(messages []conversation.Message) bool {
	if len(messages) == 0 {
		return false
	}

	start := len(messages) - 8
	if start < 0 {
		start = 0
	}
	for _, msg := range messages[start:] {
		if strings.Contains(msg.Content, "<write_file>") ||
			strings.Contains(msg.Content, "<append_file>") ||
			strings.Contains(msg.Content, "<run_command>") ||
			strings.Contains(msg.Content, "已自动保存到文件") ||
			strings.Contains(msg.Content, "命令:") {
			return true
		}
	}
	return false
}

func isLikelyCodingFollowUp(content string) bool {
	followUpHints := []string{
		"继续", "再加", "加一个", "改一下", "修一下", "优化一下", "测试一下", "跑一下", "编译一下",
		"fix", "refine", "continue", "update", "add", "test", "build", "run", "debug",
	}
	return containsAny(content, followUpHints...)
}

func isProjectScaffoldRequest(content string) bool {
	content = strings.ToLower(strings.TrimSpace(content))
	if content == "" {
		return false
	}

	createHints := []string{
		"创建", "生成", "写一个", "做一个", "实现", "搭建", "开发", "编写", "使用",
		"create", "generate", "build", "make", "implement", "scaffold", "setup",
	}
	projectHints := []string{
		"项目", "服务", "接口", "api", "程序", "脚本", "应用", "网站", "框架",
		"project", "service", "server", "api", "app", "framework",
		"nestjs", "nest", "gin", "express", "react", "vue", "next.js", "nextjs",
	}
	return containsAny(content, createHints...) && containsAny(content, projectHints...)
}

func loadRuntimePrompt(profile string) string {
	switch profile {
	case runtimeProfileEngineering:
		promptPath := filepath.Join(tools.GetWorkDir(), ".freexclaw", "prompts", "coding-runtime.md")
		skillPath := filepath.Join(tools.GetWorkDir(), ".freexclaw", "skills", "engineering-delivery", "SKILL.md")
		runtimeText := loadPromptText(promptPath, builtinEngineeringRuntimePrompt)
		skillText := summarizeSkillContent(loadPromptText(skillPath, builtinEngineeringSkillSummary))
		if strings.TrimSpace(skillText) == "" {
			return runtimeText
		}
		if strings.TrimSpace(runtimeText) == "" {
			return skillText
		}
		return runtimeText + "\n\n技能摘要：\n" + skillText
	default:
		return ""
	}
}

func loadPromptText(path string, fallback string) string {
	if cached, ok := runtimePromptCache.Load(path); ok {
		return cached.(string)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		runtimePromptCache.Store(path, fallback)
		return fallback
	}

	text := strings.TrimSpace(string(data))
	if text == "" {
		text = fallback
	}
	runtimePromptCache.Store(path, text)
	return text
}

func buildRuntimeSystemMessage(profile string, summary string) string {
	if strings.TrimSpace(summary) == "" {
		return ""
	}
	if strings.TrimSpace(profile) == "" {
		return summary
	}
	return "当前任务模式: " + profile + "\n\n" + summary
}

func loadDeliveryTemplateSummary(profile string) string {
	switch profile {
	case runtimeProfileEngineering:
		path := filepath.Join(tools.GetWorkDir(), ".freexclaw", "templates", "delivery-report.md")
		return loadPromptText(path, builtinDeliveryTemplateSummary)
	default:
		return ""
	}
}

func buildDeliverySystemMessage(profile string, touchedFiles []string, commands []string) string {
	if strings.TrimSpace(profile) == "" {
		return ""
	}
	if len(touchedFiles) == 0 && len(commands) == 0 {
		return ""
	}

	summary := loadDeliveryTemplateSummary(profile)
	if strings.TrimSpace(summary) == "" {
		return ""
	}

	summary = condenseTemplateSummary(summary)
	return "最终回答请优先使用简洁的交付报告结构：" + summary + "。如果某一项没有内容可以省略，但请明确说明实际修改文件、执行命令、通过的校验和剩余风险。"
}

func condenseTemplateSummary(content string) string {
	lines := strings.Split(content, "\n")
	var sections []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## ") {
			sections = append(sections, strings.TrimSpace(strings.TrimPrefix(line, "## ")))
		}
	}
	if len(sections) == 0 {
		return strings.TrimSpace(content)
	}
	return strings.Join(sections, " / ")
}

func summarizeSkillContent(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	var picked []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "- ") {
			picked = append(picked, strings.TrimSpace(strings.TrimPrefix(line, "- ")))
		}
		if len(picked) >= 6 {
			break
		}
	}
	if len(picked) == 0 {
		return content
	}
	return strings.Join(picked, "；")
}
