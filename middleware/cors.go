package tmiddleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CorsMiddleware returns Gin middleware that applies permissive CORS response headers,
// allows common HTTP methods and handles browser preflights. Ordinary OPTIONS
// requests continue through the handler chain.
func CorsMiddleware() gin.HandlerFunc {
	return func(ginContext *gin.Context) {
		ginContext.Header("Access-Control-Allow-Origin", "*")
		ginContext.Header("Access-Control-Allow-Headers", "Authorization, Content-Length, X-CSRF-Token, Token, session, X_Requested_With, Accept, Origin, Host, Connection, Accept-Encoding, Accept-Language, DNT, X-CustomHeader, Keep-Alive, User-Agent, X-Requested-With, If-Modified-Since, Cache-Control, Content-Type, Pragma")
		ginContext.Header("Access-Control-Allow-Methods", "POST, GET, HEAD, OPTIONS, PUT, PATCH, DELETE")
		ginContext.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Cache-Control, Content-Language, Content-Type, Expires, Last-Modified, Pragma")
		ginContext.Header("Access-Control-Max-Age", "172800")
		request := ginContext.Request

		if request.Method == http.MethodOptions && request.Header.Get("Origin") != "" && request.Header.Get("Access-Control-Request-Method") != "" {
			ginContext.AbortWithStatus(http.StatusNoContent)

			return
		}

		ginContext.Next()
	}
}
