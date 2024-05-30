package config

import (
	"encoding/json"
	"fmt"
	"lightDAG/core"
	"lightDAG/crypto"
	"lightDAG/pool"
	"os"
)

func savetoFile(filename string, data map[string]interface{}) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "\t")
	if err := encoder.Encode(data); err != nil {
		panic(err)
	}
}

func GenerateKeys(pairs int, path string) {
	for i := 0; i < pairs; i++ {
		filename := fmt.Sprintf("%s/node-key-%d.json", path, i)
		keys := make(map[string]interface{})
		pri, pub := crypto.GenED25519Keys()
		keys["public"] = string(crypto.EncodePublicKey(crypto.PublickKey{Pubkey: pub}))
		keys["private"] = string(crypto.EncodePrivateKey(crypto.PrivateKey{Prikey: pri}))
		savetoFile(filename, keys)
	}
}

func GenerateTsKeys(N, T int, path string) {
	shares, pub := crypto.GenTSKeys(T, N)
	for i := 0; i < N; i++ {
		filename := fmt.Sprintf("%s/node-ts-key-%d.json", path, i)
		keys := make(map[string]interface{})
		share, _ := crypto.EncodeTSPartialKey(shares[i])
		pub, _ := crypto.EncodeTSPublicKey(pub)
		keys["share"] = string(share)
		keys["pub"] = string(pub)
		keys["N"] = N
		keys["T"] = T
		savetoFile(filename, keys)
	}
}

func readFromFile(filename string) (map[string]interface{}, error) {
	file, err := os.OpenFile(filename, os.O_RDONLY, 0600)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	decode := json.NewDecoder(file)
	data := make(map[string]interface{})
	if err := decode.Decode(&data); err != nil {
		return nil, err
	}
	return data, nil
}

func GenKeysFromFile(filename string) (pubKey crypto.PublickKey, priKey crypto.PrivateKey, err error) {
	var data map[string]interface{}
	if data, err = readFromFile(filename); err != nil {
		return
	} else {
		pub := data["public"].(string)
		pri := data["private"].(string)
		pubKey, err = crypto.DecodePublicKey([]byte(pub))
		if err != nil {
			return
		}
		priKey, err = crypto.DecodePrivateKey([]byte(pri))
		if err != nil {
			return
		}
		return
	}
}

func GenTsKeyFromFile(filename string) (crypto.SecretShareKey, error) {

	if data, err := readFromFile(filename); err != nil {
		return crypto.SecretShareKey{}, err
	} else {
		share := data["share"].(string)
		pub := data["pub"].(string)
		N := data["N"].(float64)
		T := data["T"].(float64)
		shareKey := crypto.SecretShareKey{
			N: int(N),
			T: int(T),
		}
		shareKey.PriShare, err = crypto.DecodeTSPartialKey([]byte(share))
		if err != nil {
			return crypto.SecretShareKey{}, err
		}
		shareKey.PubPoly, err = crypto.DecodeTSPublicKey([]byte(pub))
		if err != nil {
			return crypto.SecretShareKey{}, err
		}
		return shareKey, nil
	}
}

func GenParamatersFromFile(filename string) (*pool.Parameters, *core.Parameters, error) {
	// if data, err := readFromFile(filename); err != nil {
	// 	return crypto.SecretShareKey{}, err
	// } else {
	// 	share := data["share"].(string)
	// 	pub := data["pub"].(string)
	// 	N := data["N"].(float64)
	// 	T := data["T"].(float64)
	// 	shareKey := crypto.SecretShareKey{
	// 		N: int(N),
	// 		T: int(T),
	// 	}
	// 	shareKey.PriShare, err = crypto.DecodeTSPartialKey([]byte(share))
	// 	if err != nil {
	// 		return crypto.SecretShareKey{}, err
	// 	}
	// 	shareKey.PubPoly, err = crypto.DecodeTSPublicKey([]byte(pub))
	// 	if err != nil {
	// 		return crypto.SecretShareKey{}, err
	// 	}
	// 	return shareKey, nil
	// }
	return nil, nil, nil
}

func GenCommitteeFromFile(filename string) (*core.Committee, error) {
	// if data, err := readFromFile(filename); err != nil {
	// 	return crypto.SecretShareKey{}, err
	// } else {
	// 	share := data["share"].(string)
	// 	pub := data["pub"].(string)
	// 	N := data["N"].(float64)
	// 	T := data["T"].(float64)
	// 	shareKey := crypto.SecretShareKey{
	// 		N: int(N),
	// 		T: int(T),
	// 	}
	// 	shareKey.PriShare, err = crypto.DecodeTSPartialKey([]byte(share))
	// 	if err != nil {
	// 		return crypto.SecretShareKey{}, err
	// 	}
	// 	shareKey.PubPoly, err = crypto.DecodeTSPublicKey([]byte(pub))
	// 	if err != nil {
	// 		return crypto.SecretShareKey{}, err
	// 	}
	// 	return shareKey, nil
	// }
	return nil, nil
}
