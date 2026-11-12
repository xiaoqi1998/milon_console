package api

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
)

type AccountView struct {
	Address        crypto.Address
	Threshold      uint8
	PublicKeysBs58 []string
}

func (ac *AccountView) MarshalPostcard(serializer *postcard.Serializer) error {
	if err := ac.Address.MarshalPostcard(serializer); err != nil {
		return fmt.Errorf("failed to serialize Address: %w", err)
	}

	if err := serializer.SerializeU8(ac.Threshold); err != nil {
		return fmt.Errorf("failed to serialize Threshold: %w", err)
	}

	if err := postcard.SerializeSeq(serializer, ac.PublicKeysBs58, func(s *postcard.Serializer, pk string) error {
		return s.SerializeStr(pk)
	}); err != nil {
		return fmt.Errorf("failed to serialize PublicKeysBs58: %w", err)
	}

	return nil
}

func (ac *AccountView) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	if err := ac.Address.UnmarshalPostcard(deserializer); err != nil {
		return fmt.Errorf("failed to deserialize Address: %w", err)
	}

	threshold, err := deserializer.DeserializeU8()
	if err != nil {
		return fmt.Errorf("failed to deserialize Threshold: %w", err)
	}
	ac.Threshold = threshold

	publicKeys, err := postcard.DeserializeSeq(deserializer, func(d *postcard.Deserializer) (string, error) {
		return d.DeserializeStr()
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize PublicKeysBs58: %w", err)
	}
	ac.PublicKeysBs58 = publicKeys

	return nil
}
