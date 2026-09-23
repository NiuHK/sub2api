package domain

// APIKeyGroupBinding is a key's ordered failover candidate. Priority is ascending.
type APIKeyGroupBinding struct {
	GroupID         int64 `json:"group_id"`
	Priority        int   `json:"priority"`
	CooldownSeconds int   `json:"cooldown_seconds"`
}
