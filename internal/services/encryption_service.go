package services

import (
	"winsonin/internal/crypto"
	"winsonin/internal/models"
)

// EncryptionService wraps the crypto service with domain-specific methods
type EncryptionService struct {
	crypto *crypto.EncryptionService
}

// NewEncryptionService creates a new encryption service
func NewEncryptionService(encryptionKey, blindIndexKey []byte) (*EncryptionService, error) {
	cryptoSvc, err := crypto.NewEncryptionService(encryptionKey, blindIndexKey)
	if err != nil {
		return nil, err
	}
	return &EncryptionService{crypto: cryptoSvc}, nil
}

// EncryptUser encrypts sensitive user fields before storing in DB
func (s *EncryptionService) EncryptUser(user *models.User) error {
	// Encrypt email and generate blind index
	encryptedEmail, blindIndex, err := s.crypto.EncryptWithBlindIndex(user.Email)
	if err != nil {
		return err
	}
	user.Email = encryptedEmail
	user.EmailBlindIndex = blindIndex

	// Encrypt goal if present
	if user.Goal != nil && *user.Goal != "" {
		encryptedGoal, err := s.crypto.Encrypt(*user.Goal)
		if err != nil {
			return err
		}
		user.Goal = &encryptedGoal
	}

	return nil
}

// DecryptUser decrypts sensitive user fields after retrieving from DB
func (s *EncryptionService) DecryptUser(user *models.User) error {
	// Decrypt email
	decryptedEmail, err := s.crypto.Decrypt(user.Email)
	if err != nil {
		return err
	}
	user.Email = decryptedEmail

	// Decrypt goal if present
	if user.Goal != nil && *user.Goal != "" {
		decryptedGoal, err := s.crypto.Decrypt(*user.Goal)
		if err != nil {
			return err
		}
		user.Goal = &decryptedGoal
	}

	return nil
}

// EncryptJournal encrypts sensitive journal fields before storing in DB
func (s *EncryptionService) EncryptJournal(journal *models.Journal) error {
	if journal.Topics != "" {
		encryptedTopics, err := s.crypto.Encrypt(journal.Topics)
		if err != nil {
			return err
		}
		journal.Topics = encryptedTopics
	}
	return nil
}

// DecryptJournal decrypts sensitive journal fields after retrieving from DB
func (s *EncryptionService) DecryptJournal(journal *models.Journal) error {
	if journal.Topics != "" {
		decryptedTopics, err := s.crypto.Decrypt(journal.Topics)
		if err != nil {
			return err
		}
		journal.Topics = decryptedTopics
	}
	return nil
}

// GenerateEmailBlindIndex generates a blind index for email lookup
func (s *EncryptionService) GenerateEmailBlindIndex(email string) string {
	return s.crypto.GenerateBlindIndex(email)
}
