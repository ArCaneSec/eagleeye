package models

import "time"

type Watchable struct {
	CreatedAt    time.Time
	Hash         string
	ResponseSize uint
	StatusID     uint
	Status       Status
}

type DomainMethod struct {
	DomainID     uint `gorm:"primaryKey"`
	MethodID     uint `gorm:"primaryKey"`
	Watchable
}

type DomainParameter struct {
	DomainID     uint `gorm:"primaryKey"`
	ParameterID  uint `gorm:"primaryKey"`
	Watchable
	MethodID     uint
	Method       Method
}

type EndpointMethod struct {
	EndpointID   uint `gorm:"primaryKey"`
	MethodID     uint `gorm:"primaryKey"`
	Watchable
}

type EndpointParameter struct {
	EndpointID   uint `gorm:"primaryKey"`
	ParameterID  uint `gorm:"primaryKey"`
	Watchable
	MethodID     uint
	Method       Method
}
