package api

import (
	"github.com/gin-gonic/gin"
)

func SetupRoutes() func(*gin.Engine) {
	return func(r *gin.Engine) {
		r.POST("/scan", ScanHandler)
	}
}
