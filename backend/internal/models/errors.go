package models

import "errors"

var ErrNotFound = errors.New("requested resource not found")
var ErrForbidden = errors.New("user does not have permission to access this resource")
var ErrConflict = errors.New("resource conflict, item already exists")
var ErrInactiveAccount = errors.New("user account is not active")
var ErrInvalidToken = errors.New("token not found or expired")
var ErrInvalidCredentials = errors.New("invalid credentials") // email or password provided does not match database record
var ErrInvalidForumPostCategoryID = errors.New("invalid category of forum post")
var ErrInvalidOperation = errors.New("the requested operation is not valid for the target resource")
var ErrLimitExceeded = errors.New("submission or usage limit exceeded")
var ErrMissedDeadline = errors.New("deadline missed")
var ErrNicknameTaken = errors.New("nickname already taken")
var ErrNotOwned = errors.New("not owned by this user")
