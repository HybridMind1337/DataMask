package config

// FrameworkPreset contains pre-configured column and table sanitisation rules.
type FrameworkPreset struct {
	Name        string
	Description string
	Rules       map[string]map[string]string // table -> column -> strategy
}

var frameworkPresets = map[string]FrameworkPreset{
	"laravel": {
		Name:        "laravel",
		Description: "Preset rules for standard Laravel applications (Eloquent, Sanctum, sessions, jobs)",
		Rules: map[string]map[string]string{
			"users": {
				"password":       "dev_auth",
				"remember_token": "null",
				"email":          "fake_email",
				"name":           "fake_name",
			},
			"sessions": {
				"payload": "null",
			},
			"personal_access_tokens": {
				"token": "redact",
			},
			"failed_jobs": {
				"payload":   "json_mask",
				"exception": "redact",
			},
			"password_resets": {
				"token": "redact",
			},
			"password_reset_tokens": {
				"token": "redact",
			},
		},
	},
	"django": {
		Name:        "django",
		Description: "Preset rules for standard Django applications",
		Rules: map[string]map[string]string{
			"auth_user": {
				"password":   "dev_auth",
				"email":      "fake_email",
				"first_name": "fake_first_name",
				"last_name":  "fake_last_name",
			},
			"django_session": {
				"session_data": "null",
			},
			"authtoken_token": {
				"key": "redact",
			},
		},
	},
	"rails": {
		Name:        "rails",
		Description: "Preset rules for Ruby on Rails / Devise applications",
		Rules: map[string]map[string]string{
			"users": {
				"encrypted_password":   "dev_auth",
				"reset_password_token": "null",
				"email":                "fake_email",
			},
			"active_storage_blobs": {
				"key":      "random_uuid",
				"checksum": "redact",
			},
		},
	},
	"wordpress": {
		Name:        "wordpress",
		Description: "Preset rules for standard WordPress installations",
		Rules: map[string]map[string]string{
			"wp_users": {
				"user_pass":     "dev_auth",
				"user_email":    "fake_email",
				"user_login":    "fake_username",
				"user_nicename": "fake_username",
			},
		},
	},
}

// GetPreset returns the rules for a recognized framework profile.
func GetPreset(name string) (FrameworkPreset, bool) {
	p, ok := frameworkPresets[name]
	return p, ok
}

// AvailablePresets returns list of registered preset names.
func AvailablePresets() []string {
	res := make([]string, 0, len(frameworkPresets))
	for k := range frameworkPresets {
		res = append(res, k)
	}
	return res
}
