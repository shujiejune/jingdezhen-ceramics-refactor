package models

// ContactFormData represents the data from the contact form
type ContactFormData struct {
	Name    string `json:"name" validate:"required,max=100"`
	Email   string `json:"email" validate:"required,email"`
	Subject string `json:"subject" validate:"required,max=255"`
	Message string `json:"message" validate:"required,min=10"`
}
