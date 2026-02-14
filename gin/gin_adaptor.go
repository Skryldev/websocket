package wsx

import (
    "github.com/gin-gonic/gin"
    "github.com/Skryldev/websocket"
)

func Handler(s *wsx.Server) gin.HandlerFunc {

    return func(c *gin.Context) {

        s.ServeHTTP(c.Writer, c.Request)
    }
}
