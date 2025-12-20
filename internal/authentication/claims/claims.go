package claims

import "github.com/hmcalister/LiteralCloudService/internal/database"

func UserToTokenClaims(user database.User) map[string]interface{} {
	return map[string]interface{}{
		"id":    user.UserID,
		"email": user.Email,
	}
}
