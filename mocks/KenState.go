// Code generated by mockery v2.40.1. DO NOT EDIT.

package mocks

import (
	discordgo "github.com/bwmarrin/discordgo"
	mock "github.com/stretchr/testify/mock"
)

// KenState is an autogenerated mock type for the State type
type KenState struct {
	mock.Mock
}

// Channel provides a mock function with given fields: s, id
func (_m *KenState) Channel(s *discordgo.Session, id string) (*discordgo.Channel, error) {
	ret := _m.Called(s, id)

	if len(ret) == 0 {
		panic("no return value specified for Channel")
	}

	var r0 *discordgo.Channel
	var r1 error
	if rf, ok := ret.Get(0).(func(*discordgo.Session, string) (*discordgo.Channel, error)); ok {
		return rf(s, id)
	}
	if rf, ok := ret.Get(0).(func(*discordgo.Session, string) *discordgo.Channel); ok {
		r0 = rf(s, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*discordgo.Channel)
		}
	}

	if rf, ok := ret.Get(1).(func(*discordgo.Session, string) error); ok {
		r1 = rf(s, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Guild provides a mock function with given fields: s, id
func (_m *KenState) Guild(s *discordgo.Session, id string) (*discordgo.Guild, error) {
	ret := _m.Called(s, id)

	if len(ret) == 0 {
		panic("no return value specified for Guild")
	}

	var r0 *discordgo.Guild
	var r1 error
	if rf, ok := ret.Get(0).(func(*discordgo.Session, string) (*discordgo.Guild, error)); ok {
		return rf(s, id)
	}
	if rf, ok := ret.Get(0).(func(*discordgo.Session, string) *discordgo.Guild); ok {
		r0 = rf(s, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*discordgo.Guild)
		}
	}

	if rf, ok := ret.Get(1).(func(*discordgo.Session, string) error); ok {
		r1 = rf(s, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Role provides a mock function with given fields: s, gID, id
func (_m *KenState) Role(s *discordgo.Session, gID string, id string) (*discordgo.Role, error) {
	ret := _m.Called(s, gID, id)

	if len(ret) == 0 {
		panic("no return value specified for Role")
	}

	var r0 *discordgo.Role
	var r1 error
	if rf, ok := ret.Get(0).(func(*discordgo.Session, string, string) (*discordgo.Role, error)); ok {
		return rf(s, gID, id)
	}
	if rf, ok := ret.Get(0).(func(*discordgo.Session, string, string) *discordgo.Role); ok {
		r0 = rf(s, gID, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*discordgo.Role)
		}
	}

	if rf, ok := ret.Get(1).(func(*discordgo.Session, string, string) error); ok {
		r1 = rf(s, gID, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SelfUser provides a mock function with given fields: s
func (_m *KenState) SelfUser(s *discordgo.Session) (*discordgo.User, error) {
	ret := _m.Called(s)

	if len(ret) == 0 {
		panic("no return value specified for SelfUser")
	}

	var r0 *discordgo.User
	var r1 error
	if rf, ok := ret.Get(0).(func(*discordgo.Session) (*discordgo.User, error)); ok {
		return rf(s)
	}
	if rf, ok := ret.Get(0).(func(*discordgo.Session) *discordgo.User); ok {
		r0 = rf(s)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*discordgo.User)
		}
	}

	if rf, ok := ret.Get(1).(func(*discordgo.Session) error); ok {
		r1 = rf(s)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// User provides a mock function with given fields: s, id
func (_m *KenState) User(s *discordgo.Session, id string) (*discordgo.User, error) {
	ret := _m.Called(s, id)

	if len(ret) == 0 {
		panic("no return value specified for User")
	}

	var r0 *discordgo.User
	var r1 error
	if rf, ok := ret.Get(0).(func(*discordgo.Session, string) (*discordgo.User, error)); ok {
		return rf(s, id)
	}
	if rf, ok := ret.Get(0).(func(*discordgo.Session, string) *discordgo.User); ok {
		r0 = rf(s, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*discordgo.User)
		}
	}

	if rf, ok := ret.Get(1).(func(*discordgo.Session, string) error); ok {
		r1 = rf(s, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewKenState creates a new instance of KenState. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewKenState(t interface {
	mock.TestingT
	Cleanup(func())
}) *KenState {
	mock := &KenState{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
