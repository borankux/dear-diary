package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Config 保存认证配置信息。
type Config struct {
	Password    string
	JWTSecret   string
	TokenExpiry time.Duration
}

// NewConfig 从环境变量读取认证配置。
// 如果 DIARY_PASSWORD 未设置，返回 nil（不启用认证）。
// 可选变量：DIARY_JWT_SECRET（缺失则自动生成随机 32 字节 hex）、DIARY_JWT_EXPIRY（默认 7 天）。
func NewConfig() *Config {
	password := os.Getenv("DIARY_PASSWORD")
	if password == "" {
		return nil
	}

	jwtSecret := os.Getenv("DIARY_JWT_SECRET")
	if jwtSecret == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			panic("自动生成 JWT 密钥失败: " + err.Error())
		}
		jwtSecret = hex.EncodeToString(b)
	}

	tokenExpiry := 7 * 24 * time.Hour
	if expStr := os.Getenv("DIARY_JWT_EXPIRY"); expStr != "" {
		if d, err := time.ParseDuration(expStr); err == nil {
			tokenExpiry = d
		}
	}

	return &Config{
		Password:    password,
		JWTSecret:   jwtSecret,
		TokenExpiry: tokenExpiry,
	}
}

// GenerateToken 生成包含 iat 与 exp 声明的 JWT Token，使用 HS256 签名。
func (c *Config) GenerateToken() (string, error) {
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"iat": now.Unix(),
		"exp": now.Add(c.TokenExpiry).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(c.JWTSecret))
}

// ValidateToken 验证 JWT 字符串并返回解析后的 Token。
func (c *Config) ValidateToken(tokenString string) (*jwt.Token, error) {
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return []byte(c.JWTSecret), nil
	})
}

// isPublicPath 判断请求路径是否为公开路径，无需认证。
func isPublicPath(path string) bool {
	publicPaths := []string{
		"/auth/login",
		"/health",
		"/",
		"/index.html",
	}
	for _, p := range publicPaths {
		if path == p {
			return true
		}
	}
	if strings.HasPrefix(path, "/assets/") {
		return true
	}
	return false
}

// writeJSONError 将 JSON 错误响应写入 ResponseWriter。
func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// AuthMiddleware 返回 HTTP 中间件，公开路径直接放行，其余路径需要 Bearer Token 认证。
// 如果 Config 为 nil，不启用认证（向后兼容本地模式）。
func (c *Config) AuthMiddleware(next http.Handler) http.Handler {
	if c == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		const prefix = "Bearer "
		if !strings.HasPrefix(authHeader, prefix) {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		tokenString := strings.TrimPrefix(authHeader, prefix)
		if _, err := c.ValidateToken(tokenString); err != nil {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// LoginHandler 处理登录 POST 请求，验证密码后返回 JWT Token。
// 如果认证未启用，返回 503。
func (c *Config) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if c == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "authentication not enabled")
		return
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "请求方法不允许")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "请求体解析失败")
		return
	}

	if req.Password != c.Password {
		writeJSONError(w, http.StatusUnauthorized, "invalid password")
		return
	}

	token, err := c.GenerateToken()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "生成 Token 失败")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
}
