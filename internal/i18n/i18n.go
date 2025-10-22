package i18n

import (
	"net/http"
	"strings"
)

// Language represents a supported language
type Language string

const (
	LanguageZH Language = "zh" // Chinese Simplified
	LanguageEN Language = "en" // English
)

// DefaultLanguage is the default language for the application
const DefaultLanguage = LanguageZH

// Translations holds all translation strings
var Translations = map[Language]map[string]string{
	LanguageZH: {
		// Login page
		"login.title":                "登录 - Gemini 代理",
		"login.heading":              "Gemini 代理",
		"login.subtitle":             "登录以继续",
		"login.password":             "密码",
		"login.password.placeholder": "输入密码",
		"login.signin":               "登录",
		"login.error.invalid":        "密码无效，请重试。",

		// Dashboard header
		"dashboard.title":    "Gemini 代理控制面板",
		"dashboard.heading":  "Gemini 代理",
		"dashboard.subtitle": "凭证管理",
		"dashboard.logout":   "退出登录",

		// Dashboard page
		"dashboard.page.title":    "OAuth 凭证",
		"dashboard.page.subtitle": "管理您的 Google Cloud 项目凭证",

		// Stats cards
		"stats.pro.label":     "Gemini 2.5 Pro",
		"stats.pro.footer":    "重置时间",
		"stats.total.label":   "总请求数",
		"stats.total.footer":  "重置时间",
		"stats.rpm.label":     "每分钟请求数",
		"stats.rpm.footer":    "自上次重置以来的平均值",
		"stats.active.label":  "活跃凭证",
		"stats.active.footer": "不包括已禁用",

		// Actions
		"actions.add":             "添加凭证",
		"actions.select.all":      "全选",
		"actions.deselect.all":    "取消全选",
		"actions.ban.selected":    "禁用所选",
		"actions.unban.selected":  "启用所选",
		"actions.delete.selected": "删除所选",
		"actions.selected.count":  "已选择",

		// Add credential dropdown
		"add.oauth.title":  "OAuth 流程",
		"add.oauth.desc":   "使用 Google 登录",
		"add.upload.title": "上传文件",
		"add.upload.desc":  "JSON 或 ZIP 文件",

		// Credential card
		"credential.pro.models": "Pro 模型",
		"credential.all.models": "所有模型",
		"credential.ban":        "禁用",
		"credential.unban":      "启用",
		"credential.delete":     "删除",
		"credential.banned":     "已禁用",
		"credential.error":      "上次 API 错误",

		// Upload modal
		"upload.title":     "上传凭证",
		"upload.drag":      "拖放文件到此处",
		"upload.or":        "或点击浏览",
		"upload.browse":    "浏览文件",
		"upload.info":      "支持的格式：",
		"upload.json":      ".json",
		"upload.zip":       ".zip",
		"upload.json.desc": "（单个凭证）",
		"upload.zip.desc":  "（多个凭证）",

		// Empty state
		"empty.title":   "未找到凭证",
		"empty.message": "您还没有添加任何 OAuth 凭证。<br>点击上方按钮开始。",

		// Confirmations
		"confirm.ban":             "确定要禁用项目的凭证吗：",
		"confirm.unban":           "确定要启用项目的凭证吗：",
		"confirm.delete":          "确定要删除项目的凭证吗：",
		"confirm.ban.multiple":    "确定要禁用 %d 个凭证吗？",
		"confirm.unban.multiple":  "确定要启用 %d 个凭证吗？",
		"confirm.delete.multiple": "确定要删除 %d 个凭证吗？",

		// Messages
		"message.deleting":       "正在删除凭证...",
		"message.banning":        "正在禁用凭证...",
		"message.unbanning":      "正在启用凭证...",
		"message.processing":     "处理中...",
		"message.deleted":        "凭证删除成功",
		"message.banned":         "凭证禁用成功",
		"message.unbanned":       "凭证启用成功",
		"message.uploaded":       "文件上传成功",
		"message.error":          "操作失败",
		"message.oauth.redirect": "正在重定向到 Google OAuth...",

		// Loading
		"loading.text": "处理中...",

		// Language selector
		"language.switch": "切换语言",
	},
	LanguageEN: {
		// Login page
		"login.title":                "Login - Gemini Proxy",
		"login.heading":              "Gemini Proxy",
		"login.subtitle":             "Sign in to continue",
		"login.password":             "Password",
		"login.password.placeholder": "Enter password",
		"login.signin":               "Sign In",
		"login.error.invalid":        "Invalid password. Please try again.",

		// Dashboard header
		"dashboard.title":    "Gemini Proxy Dashboard",
		"dashboard.heading":  "Gemini Proxy",
		"dashboard.subtitle": "Credential Management",
		"dashboard.logout":   "Logout",

		// Dashboard page
		"dashboard.page.title":    "OAuth Credentials",
		"dashboard.page.subtitle": "Manage your Google Cloud project credentials",

		// Stats cards
		"stats.pro.label":     "Gemini 2.5 Pro",
		"stats.pro.footer":    "Resets at",
		"stats.total.label":   "Total Requests",
		"stats.total.footer":  "Resets at",
		"stats.rpm.label":     "Requests/Min",
		"stats.rpm.footer":    "Average since last reset",
		"stats.active.label":  "Active Credentials",
		"stats.active.footer": "Excluding banned",

		// Actions
		"actions.add":             "Add Credential",
		"actions.select.all":      "Select All",
		"actions.deselect.all":    "Deselect All",
		"actions.ban.selected":    "Ban Selected",
		"actions.unban.selected":  "Unban Selected",
		"actions.delete.selected": "Delete Selected",
		"actions.selected.count":  "selected",

		// Add credential dropdown
		"add.oauth.title":  "OAuth Flow",
		"add.oauth.desc":   "Sign in with Google",
		"add.upload.title": "Upload Files",
		"add.upload.desc":  "JSON or ZIP files",

		// Credential card
		"credential.pro.models": "Pro Models",
		"credential.all.models": "All Models",
		"credential.ban":        "Ban",
		"credential.unban":      "Unban",
		"credential.delete":     "Delete",
		"credential.banned":     "Banned",
		"credential.error":      "Last API Error",

		// Upload modal
		"upload.title":     "Upload Credentials",
		"upload.drag":      "Drag and drop files here",
		"upload.or":        "or click to browse",
		"upload.browse":    "Browse Files",
		"upload.info":      "Supported formats:",
		"upload.json":      ".json",
		"upload.zip":       ".zip",
		"upload.json.desc": "(single credential)",
		"upload.zip.desc":  "(multiple credentials)",

		// Empty state
		"empty.title":   "No Credentials Found",
		"empty.message": "You haven't added any OAuth credentials yet.<br>Click the button above to get started.",

		// Confirmations
		"confirm.ban":             "Are you sure you want to ban credential for project:",
		"confirm.unban":           "Are you sure you want to unban credential for project:",
		"confirm.delete":          "Are you sure you want to delete credential for project:",
		"confirm.ban.multiple":    "Are you sure you want to ban %d credential(s)?",
		"confirm.unban.multiple":  "Are you sure you want to unban %d credential(s)?",
		"confirm.delete.multiple": "Are you sure you want to delete %d credential(s)?",

		// Messages
		"message.deleting":       "Deleting credential...",
		"message.banning":        "Banning credential(s)...",
		"message.unbanning":      "Unbanning credential(s)...",
		"message.processing":     "Processing...",
		"message.deleted":        "Credential deleted successfully",
		"message.banned":         "Credential(s) banned successfully",
		"message.unbanned":       "Credential(s) unbanned successfully",
		"message.uploaded":       "File uploaded successfully",
		"message.error":          "Operation failed",
		"message.oauth.redirect": "Redirecting to Google OAuth...",

		// Loading
		"loading.text": "Processing...",

		// Language selector
		"language.switch": "Switch Language",
	},
}

// GetLanguageFromRequest determines the language from the request
// Priority: 1. Cookie, 2. Accept-Language header, 3. Default
func GetLanguageFromRequest(r *http.Request) Language {
	// Check cookie first
	if cookie, err := r.Cookie("lang"); err == nil && cookie.Value != "" {
		lang := Language(cookie.Value)
		if _, exists := Translations[lang]; exists {
			return lang
		}
	}

	// Check Accept-Language header
	acceptLang := r.Header.Get("Accept-Language")
	if acceptLang != "" {
		// Parse Accept-Language header (simplified)
		langs := strings.Split(acceptLang, ",")
		for _, lang := range langs {
			// Extract language code (e.g., "zh-CN" -> "zh")
			langCode := strings.Split(strings.TrimSpace(lang), ";")[0]
			langCode = strings.Split(langCode, "-")[0]
			langCode = strings.ToLower(langCode)

			if langCode == "zh" {
				return LanguageZH
			} else if langCode == "en" {
				return LanguageEN
			}
		}
	}

	// Return default language
	return DefaultLanguage
}

// T translates a key for the given language
func T(lang Language, key string) string {
	if translations, ok := Translations[lang]; ok {
		if translation, ok := translations[key]; ok {
			return translation
		}
	}

	// Fallback to English if translation not found
	if lang != LanguageEN {
		if translations, ok := Translations[LanguageEN]; ok {
			if translation, ok := translations[key]; ok {
				return translation
			}
		}
	}

	// Return key if no translation found
	return key
}

// GetAllTranslations returns all translations for a given language as JSON-compatible map
func GetAllTranslations(lang Language) map[string]string {
	if translations, ok := Translations[lang]; ok {
		return translations
	}
	return Translations[DefaultLanguage]
}
