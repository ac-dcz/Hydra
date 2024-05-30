package crypto

import (
	"testing"
)

func TestPK(t *testing.T) {
	pri, pub := GenED25519Keys()
	pubKey := PublickKey{
		Pubkey: pub,
	}
	priKey := PrivateKey{
		Prikey: pri,
	}

	//1. Encode/Decode
	enPub, enPri := EncodePublicKey(pubKey), EncodePrivateKey(priKey)
	t.Logf("Pubkey: %s\n", enPub)
	t.Logf("Prikey: %s\n", enPri)

	pubKey, err := DecodePublicKey(enPub)
	if err != nil {
		t.Fatal(err)
	}
	priKey, err = DecodePrivateKey(enPri)
	if err != nil {
		t.Fatal(err)
	}

	//2.Sign/Verify
	msg := []byte("dczhahahah")
	digest := NewHasher().Sum256(msg)
	srvc := NewSigService(priKey, SecretShareKey{})
	sig, err := srvc.RequestSignature(digest)
	if err != nil {
		t.Fatal(err)
	}
	if !sig.Verify(pubKey, digest) {
		t.Fatalf("sign fail")
	}
}

func TestTsPk(t *testing.T) {
	shares, pub := GenTSKeys(3, 4)
	var shareKeys []SecretShareKey
	for _, share := range shares {
		shareKeys = append(shareKeys, SecretShareKey{
			PubPoly:  pub,
			PriShare: share,
			N:        4,
			T:        3,
		})
	}
	var (
		cnt        = 0
		shareSigch = make(chan SignatureShare, 4)
	)
	msg := []byte("dczhahahah")
	digest := NewHasher().Sum256(msg)

	for i := 0; i < 4; i++ {
		ind := i
		//1. Decode/Encode
		byt, err := EncodeTSPartialKey(shareKeys[ind].PriShare)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("[%d] share %s\n", ind, byt)

		share, err := DecodeTSPartialKey(byt)
		if err != nil {
			t.Fatal(err)
		}
		if share.String() != shareKeys[ind].PriShare.String() {
			t.Fatal("encode/decode error")
		}

		//2. Sign
		srvc := NewSigService(PrivateKey{}, shareKeys[ind])
		sigShare, err := srvc.RequestTsSugnature(digest)
		if err != nil {
			t.Fatal(err)
		}
		shareSigch <- sigShare
	}

	var sigs []SignatureShare
	for sig := range shareSigch {
		sigs = append(sigs, sig)
		cnt++
		if cnt == 3 {
			break
		}
	}
	combineSig, err := CombineIntactTSPartial(sigs, shareKeys[0], digest)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyTs(shareKeys[0], digest, combineSig); err != nil {
		t.Fatal(err)
	}
}
