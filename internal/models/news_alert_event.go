package models

// NewsAlertEvent представляет событие совпадения новости с правилом алертинга.
type NewsAlertEvent struct {
	EventID        string `json:"eventId"`
	RuleID         string `json:"ruleId"`
	UserUUID       string `json:"userUuid"`
	NewsID         string `json:"newsId"`
	Keyword        string `json:"keyword"`
	MatchedField   string `json:"matchedField"`
	MatchedSnippet string `json:"matchedSnippet"`
	NewsTitle      string `json:"newsTitle"`
	NewsURL        string `json:"newsUrl"`
	CreatedAt      string `json:"createdAt"`
}
