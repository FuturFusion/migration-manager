package auth

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type objectSuite struct {
	suite.Suite
}

func TestObjectSuite(t *testing.T) {
	suite.Run(t, new(objectSuite))
}

func (s *objectSuite) TestObjectServer() {
	s.NotPanics(func() {
		o := ObjectServer()
		s.Equal("server:migration-manager", string(o))
	})
}

func (s *objectSuite) TestObjectUser() {
	s.NotPanics(func() {
		o := ObjectUser("username")
		s.Equal("user:username", string(o))
	})
}
