package config

import (
	"WuKong/core"
	"WuKong/crypto"
	"WuKong/pool"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// savetoFile save data to filename in json style
func savetoFile(filename string, data interface{}) {
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

// GenerateKeyFiles generate keys file
func GenerateKeyFiles(pairs int, path string) {
	for i := 0; i < pairs; i++ {
		filename := fmt.Sprintf("%s/.node-key-%d.json", path, i)
		keys := make(map[string]interface{})
		pri, pub := crypto.GenED25519Keys()
		keys["public"] = string(crypto.EncodePublicKey(crypto.PublickKey{Pubkey: pub}))
		keys["private"] = string(crypto.EncodePrivateKey(crypto.PrivateKey{Prikey: pri}))
		savetoFile(filename, keys)
	}
}

// GenerateTsKeyFiles generate ts keys file
func GenerateTsKeyFiles(N, T int, path string) {
	shares, pub := crypto.GenTSKeys(T, N)
	for i := 0; i < N; i++ {
		filename := fmt.Sprintf("%s/.node-ts-key-%d.json", path, i)
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

// readFromFile read json file
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

// GenKeysFromFile generate keys from json file
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

// GenTsKeyFromFile generate ts keys from json file
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

// GenParamatersFromFile generate parameters from json file
func GenParamatersFromFile(filename string) (pool.Parameters, core.Parameters, error) {
	parameters := Parameters{}
	file, err := os.OpenFile(filename, os.O_RDONLY, 0600)
	if err != nil {
		return pool.Parameters{}, core.Parameters{}, err
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(&parameters); err != nil {
		return pool.Parameters{}, core.Parameters{}, err
	}
	return parameters.Pool, parameters.Consensus, nil
}

// GenCommitteeFromFile generate committee from json file
func GenCommitteeFromFile(filename string) (core.Committee, error) {
	if data, err := readFromFile(filename); err != nil {
		return core.Committee{}, err
	} else {
		committee := core.Committee{
			Authorities: make(map[core.NodeID]core.Authority),
		}
		for id, item := range data {
			ID, _ := strconv.Atoi(id)
			info := item.(map[string]interface{})
			name, _ := crypto.DecodePublicKey([]byte(info["name"].(string)))
			committee.Authorities[core.NodeID(ID)] = core.Authority{
				Addr: info["addr"].(string),
				Id:   core.NodeID(ID),
				Name: name,
			}
		}
		return committee, nil
	}
}

func GenerateSmapleCommittee() {
	committee, _, _ := GenDefaultCommittee(4)
	data := make(map[string]interface{})
	for id, item := range committee.Authorities {
		data[strconv.Itoa(int(id))] = map[string]interface{}{
			"name":    string(crypto.EncodePublicKey(item.Name)),
			"node_id": id,
			"addr":    item.Addr,
		}
	}
	savetoFile("./.committee.json", data)
}

// GenerateSampleParameters generate parameters sample file
func GenerateSampleParameters() {
	parameters := GenDefaultParameters()
	savetoFile("./.parameters.json", parameters)
}

type Parameters struct {
	Pool      pool.Parameters `json:"pool"`
	Consensus core.Parameters `json:"consensus"`
}

const DefaultPort int = 9000

// GenDefaultParameters generate default parameters
func GenDefaultParameters() Parameters {
	return Parameters{
		Pool:      pool.DefaultParameters,
		Consensus: core.DefaultParameters,
	}
}

// GenDefaultCommittee generate default commitee,PrivateKey,SecretShareKey
func GenDefaultCommittee(n int) (core.Committee, []crypto.PrivateKey, []crypto.SecretShareKey) {
	committee := core.Committee{
		Authorities: make(map[core.NodeID]core.Authority),
	}
	priKeys, shareKeys := make([]crypto.PrivateKey, 0), make([]crypto.SecretShareKey, 0)
	for i := 0; i < n; i++ {
		priKey, pubKey := crypto.GenED25519Keys()
		committee.Authorities[core.NodeID(i)] = core.Authority{
			Name: crypto.PublickKey{
				Pubkey: pubKey,
			},
			Id:   core.NodeID(i),
			Addr: fmt.Sprintf("127.0.0.1:%d", DefaultPort+i),
		}
		priKeys = append(priKeys, crypto.PrivateKey{Prikey: priKey})
	}
	shares, pub := crypto.GenTSKeys(committee.HightThreshold(), n)
	for i := 0; i < n; i++ {
		shareKeys = append(shareKeys, crypto.SecretShareKey{
			PubPoly:  pub,
			PriShare: shares[i],
			N:        n,
			T:        committee.HightThreshold(),
		})
	}
	return committee, priKeys, shareKeys
}
