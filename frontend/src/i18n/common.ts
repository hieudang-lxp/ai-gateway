const en = {
  tagline: "Your AI workspace, in view.",
  overview: "Overview", sessions: "Sessions", insights: "Insights", "local-memory": "Local memory", "data-pricing": "Data & pricing",
  networkTitle: "Network explorer", skipContent: "Skip to content", home: "AI Gateway overview", navigation: "Main navigation",
  language: "Language", currency: "Currency", usageStatus: "Usage collection: {{status}}", usageSynced: "Usage updated {{time}}",
  rateUpdated: "Exchange rate updated {{time}}", rateUnavailable: "Exchange rate unavailable", exchangeRate: "1 USD = {{value}} VND",
  overviewDescription: "Usage across Claude Code, Codex and Cursor.", proxyTitle: "Gateway diagnostics", proxyDescription: "Proxy requests only. Includes budgets, cache and request history.",
  close: "Close", commands: "Commands", searchCommands: "Find a command…", dashboardToken: "Dashboard token", bearerToken: "Bearer token", save: "Save",
  unexpectedError: "This view could not be loaded.", errorDetails: "Technical details", retry: "Try again",
} as const;
type Dictionary = Record<keyof typeof en, string>;
const vi: Dictionary = {
  tagline: "Không gian AI của bạn, trong tầm mắt.",
  overview: "Tổng quan", sessions: "Phiên làm việc", insights: "Phân tích", "local-memory": "Bộ nhớ cục bộ", "data-pricing": "Dữ liệu & giá",
  networkTitle: "Khám phá mạng lưới", skipContent: "Đến nội dung chính", home: "Tổng quan AI Gateway", navigation: "Điều hướng chính",
  language: "Ngôn ngữ", currency: "Tiền tệ", usageStatus: "Thu thập sử dụng: {{status}}", usageSynced: "Sử dụng cập nhật lúc {{time}}",
  rateUpdated: "Tỷ giá cập nhật lúc {{time}}", rateUnavailable: "Chưa có tỷ giá", exchangeRate: "1 USD = {{value}} VND",
  overviewDescription: "Mức sử dụng Claude Code, Codex và Cursor.", proxyTitle: "Chẩn đoán gateway", proxyDescription: "Chỉ tính yêu cầu qua proxy. Gồm ngân sách, bộ nhớ đệm và lịch sử yêu cầu.",
  close: "Đóng", commands: "Lệnh", searchCommands: "Tìm lệnh…", dashboardToken: "Token truy cập dashboard", bearerToken: "Bearer token", save: "Lưu",
  unexpectedError: "Không thể tải màn hình này.", errorDetails: "Chi tiết kỹ thuật", retry: "Thử lại",
};
const ko: Dictionary = {
  tagline: "AI 작업 공간을 한눈에.",
  overview: "개요", sessions: "세션", insights: "분석", "local-memory": "로컬 메모리", "data-pricing": "데이터 및 요금",
  networkTitle: "네트워크 탐색", skipContent: "본문으로 이동", home: "AI Gateway 개요", navigation: "기본 탐색",
  language: "언어", currency: "통화", usageStatus: "사용량 수집: {{status}}", usageSynced: "사용량 업데이트 {{time}}",
  rateUpdated: "환율 업데이트 {{time}}", rateUnavailable: "환율을 불러올 수 없음", exchangeRate: "1 USD = {{value}} VND",
  overviewDescription: "Claude Code, Codex, Cursor의 사용량입니다.", proxyTitle: "게이트웨이 진단", proxyDescription: "프록시 요청만 포함합니다. 예산, 캐시 및 요청 기록을 확인하세요.",
  close: "닫기", commands: "명령", searchCommands: "명령 찾기…", dashboardToken: "대시보드 토큰", bearerToken: "Bearer 토큰", save: "저장",
  unexpectedError: "이 화면을 불러올 수 없습니다.", errorDetails: "기술 세부 정보", retry: "다시 시도",
};
const zh: Dictionary = {
  tagline: "你的 AI 工作空间，一目了然。",
  overview: "概览", sessions: "会话", insights: "分析", "local-memory": "本地记忆", "data-pricing": "数据与价格",
  networkTitle: "网络探索", skipContent: "跳转到主要内容", home: "AI Gateway 概览", navigation: "主导航",
  language: "语言", currency: "货币", usageStatus: "用量采集：{{status}}", usageSynced: "用量更新于 {{time}}",
  rateUpdated: "汇率更新于 {{time}}", rateUnavailable: "暂无汇率", exchangeRate: "1 USD = {{value}} VND",
  overviewDescription: "Claude Code、Codex 和 Cursor 的用量。", proxyTitle: "网关诊断", proxyDescription: "仅统计代理请求，包括预算、缓存和请求记录。",
  close: "关闭", commands: "命令", searchCommands: "查找命令…", dashboardToken: "控制台令牌", bearerToken: "Bearer 令牌", save: "保存",
  unexpectedError: "无法加载此页面。", errorDetails: "技术详情", retry: "重试",
};
const de: Dictionary = {
  tagline: "Dein KI-Arbeitsplatz im Überblick.",
  overview: "Übersicht", sessions: "Sitzungen", insights: "Analyse", "local-memory": "Lokaler Speicher", "data-pricing": "Daten & Preise",
  networkTitle: "Netzwerkansicht", skipContent: "Zum Inhalt springen", home: "AI Gateway Übersicht", navigation: "Hauptnavigation",
  language: "Sprache", currency: "Währung", usageStatus: "Nutzungserfassung: {{status}}", usageSynced: "Nutzung aktualisiert: {{time}}",
  rateUpdated: "Wechselkurs aktualisiert: {{time}}", rateUnavailable: "Wechselkurs nicht verfügbar", exchangeRate: "1 USD = {{value}} VND",
  overviewDescription: "Nutzung von Claude Code, Codex und Cursor.", proxyTitle: "Gateway-Diagnose", proxyDescription: "Nur Proxy-Anfragen. Budgets, Cache und Anfrageverlauf.",
  close: "Schließen", commands: "Befehle", searchCommands: "Befehl suchen…", dashboardToken: "Dashboard-Token", bearerToken: "Bearer-Token", save: "Speichern",
  unexpectedError: "Diese Ansicht konnte nicht geladen werden.", errorDetails: "Technische Details", retry: "Erneut versuchen",
};
export const commonLocales = { en, vi, ko, "zh-Hans": zh, de };
