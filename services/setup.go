// services/setup.go

type SetupInput struct {
    // Company info
    BusinessName string  `json:"business_name"`
    BusinessType string  `json:"business_type"`
    Phone        *string `json:"phone"`
    Address      *string `json:"address"`
    TIN          *string `json:"tin"`

    // First admin user
    OwnerName     string `json:"owner_name"`
    OwnerEmail    string `json:"owner_email"`
    OwnerPassword string `json:"owner_password"`
}