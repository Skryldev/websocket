package wsx

import (
	"github.com/Skryldev/websocket"
	"github.com/gin-gonic/gin"
)

func Handler(s *wsx.Server) gin.HandlerFunc {

	return func(c *gin.Context) {

		s.ServeHTTP(c.Writer, c.Request)
	}
}
