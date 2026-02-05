package endpoint

import "strings"

func writeTSBanner(b *strings.Builder, title string) {
	b.WriteString("/**\n")
	b.WriteString(" * =====================================================\n")
	b.WriteString(" * ")
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(" * -----------------------------------------------------\n")
	b.WriteString(" * [EN] This file is auto-generated. Do not edit by hand.\n")
	b.WriteString(" * [EN] Regenerate by running the Go server endpoint export.\n")
	b.WriteString(" * [EN] Edits will be overwritten on the next generation.\n")
	b.WriteString(" * -----------------------------------------------------\n")
	b.WriteString(" * [中文] 本文件由工具自动生成，请勿手动修改。\n")
	b.WriteString(" * [中文] 如需更新，请通过 Go 服务端重新生成。\n")
	b.WriteString(" * [中文] 手动修改将在下次生成时被覆盖。\n")
	b.WriteString(" * =====================================================\n")
	b.WriteString(" */\n\n")
}
