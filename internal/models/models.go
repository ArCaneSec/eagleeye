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
	Name string `gorm:"uniqueIndex;not null;size:255;"`
}

type Email struct {
	SimpleModel
	Email     string  `gorm:"uniqueIndex;not null;size:255;"`
	CompanyID uint    `gorm:"index;not null;"`
	Company   Company `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type Domain struct {
	SimpleModel
	Domain     string      `gorm:"uniqueIndex;not null;size:255;"`
	Methods    []Method    `gorm:"many2many:domain_methods;"`
	Parameters []Parameter `gorm:"many2many:domain_parameters;"`
	CompanyID  uint        `gorm:"index;not null;"`
	Company    Company     `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type Parameter struct {
	SimpleModel
	Parameter string   `gorm:"uniqueIndex;not null;size:255;"`
	Domains   []Domain `gorm:"many2many:domain_parameters;"`
	Endpoints   []Domain `gorm:"many2many:endpoint_parameters;"`
}

type Endpoint struct {
	SimpleModel
	Path       string      `gorm:"uniqueIndex:unique_path_param;not null;size:255;"`
	Methods    []Method    `gorm:"many2many:endpoint_methods;"`
	Parameters []Parameter `gorm:"many2many:endpoint_parameters;"`
	DomainID   uint        `gorm:"uniqueIndex:unique_path_param;not null;"`
	Domain     Domain      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type Status struct {
	ID     uint `gorm:"primaryKey"`
	Status uint16
}

type Method struct {
	ID     uint `gorm:"primaryKey"`
	Method string
}
