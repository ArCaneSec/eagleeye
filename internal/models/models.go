package models

import (
	"time"
)

type SimpleModel struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Company struct {
	SimpleModel
	Name string
}

type Email struct {
	SimpleModel
	Email     string
	CompanyID uint
	Company   Company
}

type Domain struct {
	SimpleModel
	Domain       string
	Methods      []Method    `gorm:"many2many:domain_method;"`
	Parameters   []Parameter `gorm:"many2many:domain_parameter;"`
	CompanyID    uint
	Company      Company
}

type Parameter struct {
	SimpleModel
	Parameter string
	Domains   []Domain `gorm:"many2many:domain_parameter;"`
}

type Status struct {
	ID     uint `gorm:"primaryKey"`
	Status uint16
}

type Method struct {
	ID     uint `gorm:"primaryKey"`
	Method string
}

type Endpoint struct {
	SimpleModel
	Path         string
	Methods      []Method    `gorm:"many2many:endpoint_method;"`
	Parameters   []Parameter `gorm:"many2many:endpoint_parameter;"`
	DomainID     uint
	Domain       Domain
}
