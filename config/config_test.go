package config

import (
	"WuKong/crypto"
	"fmt"
	"testing"
)

func TestGenerateKeys(t *testing.T) {
	GenerateKeyFiles(4, "./")
}

func TestGenrateTsKeys(t *testing.T) {
	GenerateTsKeyFiles(4, 3, "./")
}

func TestFromFileGenKey(t *testing.T) {
	filename := "./.node-key-0.json"
	pub, pri, err := GenKeysFromFile(filename)
	if err != nil {
		t.Fatal(err)
	}

	srvc := crypto.NewSigService(pri, crypto.SecretShareKey{})
	d := crypto.NewHasher().Sum256([]byte("dcz"))
	sig, _ := srvc.RequestSignature(d)
	if !sig.Verify(pub, d) {
		t.Fatalf("error")
	}
}

func TestFromFileGenTsKey(t *testing.T) {
	var shareKeys []crypto.SecretShareKey
	for i := 0; i < 4; i++ {
		filename := fmt.Sprintf("./.node-ts-key-%d.json", i)
		shareKey, err := GenTsKeyFromFile(filename)
		if err != nil {
			t.Fatal(err)
		}
		shareKeys = append(shareKeys, shareKey)
	}

	var (
		cnt        = 0
		shareSigch = make(chan crypto.SignatureShare, 4)
	)

	msg := []byte("dczhahahah")
	digest := crypto.NewHasher().Sum256(msg)

	for i := 0; i < 4; i++ {
		ind := i
		//1. Decode/Encode
		byt, err := crypto.EncodeTSPartialKey(shareKeys[ind].PriShare)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("[%d] share %s\n", ind, byt)

		share, err := crypto.DecodeTSPartialKey(byt)
		if err != nil {
			t.Fatal(err)
		}
		if share.String() != shareKeys[ind].PriShare.String() {
			t.Fatal("encode/decode error")
		}

		//2. Sign
		srvc := crypto.NewSigService(crypto.PrivateKey{}, shareKeys[ind])
		sigShare, err := srvc.RequestTsSugnature(digest)
		if err != nil {
			t.Fatal(err)
		}
		shareSigch <- sigShare
	}

	var sigs []crypto.SignatureShare
	for sig := range shareSigch {
		sigs = append(sigs, sig)
		cnt++
		if cnt == 3 {
			break
		}
	}
	combineSig, err := crypto.CombineIntactTSPartial(sigs, shareKeys[0], digest)
	if err != nil {
		t.Fatal(err)
	}
	if err := crypto.VerifyTs(shareKeys[0], digest, combineSig); err != nil {
		t.Fatal(err)
	}
}

func TestSmapleParameters(t *testing.T) {
	GenerateSampleParameters()
}

func TestSmapleCommittee(t *testing.T) {
	GenerateSmapleCommittee()
}

func TestGenParametersFromFile(t *testing.T) {
	poolP, coreP, err := GenParamatersFromFile("./.parameters.json")
	if err != nil {
		t.Fatal(err)
	} else {
		t.Log(poolP)
		t.Log(coreP)
	}
}

func TestGenCommitteeFromFile(t *testing.T) {
	committee, err := GenCommitteeFromFile("./.committee.json")
	if err != nil {
		t.Fatal(err)
	} else {
		t.Log(committee)
	}
}
