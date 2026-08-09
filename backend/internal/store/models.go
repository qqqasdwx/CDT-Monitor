package store

import "time"

type Account struct {
	ID                       string    `json:"id"`
	AccessKeyID              string    `json:"accessKeyId"`
	AccessKeySecretEncrypted string    `json:"-"`
	Region                   string    `json:"region"`
	Name                     string    `json:"name"`
	Enabled                  bool      `json:"enabled"`
	CreatedAt                time.Time `json:"createdAt"`
	UpdatedAt                time.Time `json:"updatedAt"`
}

type Instance struct {
	ID                       string     `json:"id"`
	AccountID                string     `json:"accountId"`
	InstanceID               string     `json:"instanceId"`
	Name                     string     `json:"name"`
	TrafficLimitBytes        int64      `json:"trafficLimitBytes"`
	StopMode                 string     `json:"stopMode"`
	Enabled                  bool       `json:"enabled"`
	ProtectionEnabled        bool       `json:"protectionEnabled"`
	KeepaliveEnabled         bool       `json:"keepaliveEnabled"`
	KeepaliveCooldownSeconds int64      `json:"keepaliveCooldownSeconds"`
	MonthlyRestoreEnabled    bool       `json:"monthlyRestoreEnabled"`
	LastStatus               string     `json:"lastStatus"`
	LastTrafficBytes         int64      `json:"lastTrafficBytes"`
	LastSyncedAt             *time.Time `json:"lastSyncedAt,omitempty"`
	StoppedByProtectionAt    *time.Time `json:"stoppedByProtectionAt,omitempty"`
	ManualStopAt             *time.Time `json:"manualStopAt,omitempty"`
	LastKeepaliveAt          *time.Time `json:"lastKeepaliveAt,omitempty"`
	CreatedAt                time.Time  `json:"createdAt"`
	UpdatedAt                time.Time  `json:"updatedAt"`
	AccountName              string     `json:"accountName,omitempty"`
	Region                   string     `json:"region,omitempty"`
}

type TrafficSnapshot struct {
	ID          string    `json:"id"`
	AccountID   string    `json:"accountId"`
	PeriodStart time.Time `json:"periodStart"`
	PeriodEnd   time.Time `json:"periodEnd"`
	TotalBytes  int64     `json:"totalBytes"`
	CollectedAt time.Time `json:"collectedAt"`
}

type TrafficPoint struct {
	BucketStart string `json:"bucketStart"`
	TotalBytes  int64  `json:"totalBytes"`
}

type CostSnapshot struct {
	ID              string    `json:"id"`
	AccountID       string    `json:"accountId"`
	AccountName     string    `json:"accountName,omitempty"`
	AvailableAmount float64   `json:"availableAmount"`
	CreditAmount    float64   `json:"creditAmount"`
	Currency        string    `json:"currency"`
	CollectedAt     time.Time `json:"collectedAt"`
}

type InstanceStatusPoint struct {
	ID         string    `json:"id"`
	InstanceID string    `json:"instanceId"`
	Status     string    `json:"status"`
	ObservedAt time.Time `json:"observedAt"`
}

type ActionLog struct {
	ID             string    `json:"id"`
	AccountID      string    `json:"accountId,omitempty"`
	InstanceID     string    `json:"instanceId,omitempty"`
	ActionType     string    `json:"actionType"`
	TriggerSource  string    `json:"triggerSource"`
	Result         string    `json:"result"`
	Reason         string    `json:"reason"`
	TrafficBytes   *int64    `json:"trafficBytes,omitempty"`
	ThresholdBytes *int64    `json:"thresholdBytes,omitempty"`
	StopMode       string    `json:"stopMode,omitempty"`
	ErrorMessage   string    `json:"errorMessage,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

type CloudEvent struct {
	ID            string    `json:"id"`
	EventID       string    `json:"eventId"`
	InstanceID    string    `json:"instanceId"`
	EventTime     time.Time `json:"eventTime"`
	Status        string    `json:"status"`
	RawPayload    string    `json:"rawPayload"`
	ProcessStatus string    `json:"processStatus"`
	ErrorMessage  string    `json:"errorMessage,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

type Dashboard struct {
	Accounts    []Account   `json:"accounts"`
	Instances   []Instance  `json:"instances"`
	ActionLogs  []ActionLog `json:"actionLogs"`
	WebhookURL  string      `json:"webhookUrl"`
	GeneratedAt time.Time   `json:"generatedAt"`
}

type Setting struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type NotificationChannel struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Enabled    bool      `json:"enabled"`
	ConfigJSON string    `json:"configJson,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type NotificationLog struct {
	ID           string    `json:"id"`
	ChannelID    string    `json:"channelId,omitempty"`
	EventType    string    `json:"eventType"`
	Target       string    `json:"target"`
	Result       string    `json:"result"`
	ErrorMessage string    `json:"errorMessage,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

type ScheduledTask struct {
	ID               string     `json:"id"`
	InstanceConfigID string     `json:"instanceConfigId"`
	Action           string     `json:"action"`
	RunAt            time.Time  `json:"runAt"`
	Enabled          bool       `json:"enabled"`
	LastRunAt        *time.Time `json:"lastRunAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	InstanceID       string     `json:"instanceId,omitempty"`
}

type CloudflareCredential struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	APITokenEncrypted string    `json:"-"`
	Enabled           bool      `json:"enabled"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type DNSRecord struct {
	ID           string    `json:"id"`
	CredentialID string    `json:"credentialId"`
	ZoneID       string    `json:"zoneId"`
	RecordID     string    `json:"recordId"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	CurrentValue string    `json:"currentValue"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}
