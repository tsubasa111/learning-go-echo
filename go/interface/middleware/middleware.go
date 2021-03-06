package middleware

import (
	"fmt"
	"learning-go-echo/infrastructure/auth"
	"learning-go-echo/interface/context"
	"learning-go-echo/interface/validation"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

//- Next.jsをホスティングしているの場所にアクセスできるようにCORSを設定
func CORSMiddleware() echo.MiddlewareFunc {
	return middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{os.Getenv("CORS_URL")},
		AllowHeaders: []string{
			echo.HeaderAccessControlAllowHeaders,
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderXCSRFToken,
			echo.HeaderXRequestedWith,
		},
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPut,
			http.MethodPost,
			http.MethodDelete,
		},
		AllowCredentials: true,
	})
}

//- csrf
func CSRFMiddleware() echo.MiddlewareFunc {
	return middleware.CSRFWithConfig(middleware.CSRFConfig{
		TokenLookup:    "header:X-CSRF-TOKEN",
		CookieDomain:   os.Getenv("CORS_DOMAIN"),
		CookieName:     "csrf",
		CookieSecure:   true,
		CookieHTTPOnly: true,
	})
}

//- ユーザー側の認証
/*
 * authName 認証するユーザータイプ user | artist | admin
 */
func AuthMiddleware(authenticate auth.AuthInterface, token auth.TokenInterface, authorType string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return context.CastContext(func(c *context.CustomContext) error {
			err := isJwtAuth(authenticate, token, c, authorType)
			if err != nil {
				return err
			}
			return next(c)
		})
	}
}

func isJwtAuth(authenticate auth.AuthInterface, token auth.TokenInterface, c *context.CustomContext, tokenName string) error {
	c.SetAuthorID(0)

	tokens := c.GetCookieToken(tokenName)
	fmt.Println(tokens)
	//- エラーではなかったら
	if ok := auth.TokenValid(tokens.AccessToken); ok {
		//- エラーではなかったら
		if claims, err := auth.FetchTokenClaims(tokens.AccessToken); err == nil {
			//- エラーではなかったら
			if ok := authenticate.AuthValid(claims["access_uuid"].(string)); ok {
				if ok := token.IsSameAuthorType(claims["author_type"].(string)); !ok {
					return echo.NewHTTPError(http.StatusUnauthorized, map[string]interface{}{
						"data": map[string]string{
							"error": "author_typeエラー",
						},
						"status": "ng",
					})
				}
				intID := int(claims["author_id"].(float64))
				c.SetAuthorID(uint(intID))
				return nil
			}
		}
	}
	if ok := auth.RefreshTokenValid(tokens.RefreshToken); !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, map[string]interface{}{
			"data": map[string]string{
				"error": "リフレッシュトークンに不正な値が含まれています。",
			},
			"status": "ng",
		})
	}
	claims, err := auth.FetchRefreshTokenClaims(tokens.RefreshToken)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, map[string]interface{}{
			"data": map[string]string{
				"error": "リフレッシュトークンのデータを取得できませんでした。",
			},
			"status": "ng",
		})
	}
	if ok := authenticate.AuthValid(claims["refresh_uuid"].(string)); !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, map[string]interface{}{
			"data": map[string]string{
				"error": "このリフレッシュトークンは使用できません。",
			},
			"status": "ng",
		})
	}
	if ok := token.IsSameAuthorType(claims["author_type"].(string)); !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, map[string]interface{}{
			"data": map[string]string{
				"error": "author_typeエラー",
			},
			"status": "ng",
		})
	}
	authorId := claims["author_id"].(float64)
	cToken, err := token.CreateToken(int(authorId))
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, map[string]interface{}{
			"data": map[string]string{
				"error": "トークン生成エラー",
			},
			"status": "ng",
		})
	}
	if err := authenticate.CreateAuth(int(authorId), cToken); err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, map[string]interface{}{
			"data": map[string]string{
				"error": "トークンの保存に失敗しました。",
			},
			"status": "ng",
		})
	}
	authenticate.DeleteRemainingToken(claims["refresh_uuid"].(string))
	intID := int(authorId)
	c.SetAuthorID(uint(intID))
	c.SetCookieToken(cToken.AccessToken, cToken.RefreshToken, tokenName)

	return nil
}

func CustomContextMiddleware(vwd validation.ValidatorWithDB) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cc := &context.CustomContext{Context: c, Vwd: vwd}
			return next(cc)
		}
	}
}
