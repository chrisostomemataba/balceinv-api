package models

import "time"

type Settings struct {
    ID        uint `gorm:"primaryKey;autoIncrement" json:"id"`
    CompanyID uint `gorm:"uniqueIndex;not null"     json:"company_id"`

    TaxRate        float64 `gorm:"column:tax_rate;default:18.0"         json:"tax_rate"`
    Currency       string  `gorm:"default:TZS"                          json:"currency"`
    CurrencySymbol string  `gorm:"column:currency_symbol;default:TZS"   json:"currency_symbol"`

    DateFormat          string `gorm:"column:date_format;default:DD/MM/YYYY"                    json:"date_format"`
    ReceiptNumberFormat string `gorm:"column:receipt_number_format;default:SALE-{DATE}-{COUNTER}" json:"receipt_number_format"`

    EFDEnabled      bool       `gorm:"column:efd_enabled;default:false"      json:"efd_enabled"`
    EFDEndpoint     *string    `gorm:"column:efd_endpoint;default:null"      json:"efd_endpoint"`
    EFDApiKey       *string    `gorm:"column:efd_api_key;default:null"       json:"efd_api_key"`
    EFDLastTestDate *time.Time `gorm:"column:efd_last_test_date;default:null" json:"efd_last_test_date"`
    EFDTestStatus   *string    `gorm:"column:efd_test_status;default:null"   json:"efd_test_status"`

    LowStockThreshold         int     `gorm:"column:low_stock_threshold;default:5"             json:"low_stock_threshold"`
    EmailNotificationsEnabled bool    `gorm:"column:email_notifications_enabled;default:false" json:"email_notifications_enabled"`
    NotificationEmail         *string `gorm:"column:notification_email;default:null"           json:"notification_email"`
    AlertSoundEnabled         bool    `gorm:"column:alert_sound_enabled;default:true"          json:"alert_sound_enabled"`

    AlertOnLowStock   bool `gorm:"column:alert_on_low_stock;default:true"    json:"alert_on_low_stock"`
    AlertOnOutOfStock bool `gorm:"column:alert_on_out_of_stock;default:true" json:"alert_on_out_of_stock"`
    AlertOnDeadStock  bool `gorm:"column:alert_on_dead_stock;default:false"  json:"alert_on_dead_stock"`
    DeadStockDays     int  `gorm:"column:dead_stock_days;default:30"         json:"dead_stock_days"`

    PrintReceiptAutomatically bool `gorm:"column:print_receipt_automatically;default:false" json:"print_receipt_automatically"`
    ShowTaxOnReceipt          bool `gorm:"column:show_tax_on_receipt;default:true"          json:"show_tax_on_receipt"`
    ShowBarcodesOnReceipt     bool `gorm:"column:show_barcodes_on_receipt;default:false"    json:"show_barcodes_on_receipt"`

    UpdatedBy *uint     `gorm:"column:updated_by;default:null" json:"updated_by"`
    CreatedAt time.Time `gorm:"autoCreateTime"                 json:"created_at"`
    UpdatedAt time.Time `gorm:"autoUpdateTime"                 json:"updated_at"`

    Company Company `gorm:"foreignKey:CompanyID" json:"company,omitempty"`
}

func (Settings) TableName() string { return "settings" }