package utils

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
)

func TestNewMId(t *testing.T) {
	testCasesString := `7f|bafk4bzaciduswqo5oz7hdvifqbn4wcog7wz7bfklg6yg5hgxd2yjlcnhnwlegyddyph5a2466wdrcjdvbogrxrivqjmcr6l4mgpmvoahus2n3hf7
cb03|bafk4bzacidx5a2zu2jgsohtd2rmkbm2of5wi6xf56yoslpo4qbyhmh4ix6lhcs7wyjz4w665l2wguboq3qejfnqtm4zpotdd5kwvcn4clbr2nxjy
c90eaa|bafk4bzacidue2kn4tqwpszwpacn34cakrgo5vqmbxxae4rwjd2kyg24efuumwvm7vgeuqxfyjc4a62e23kmimuq7a2nw4k4n3qfbqrq42uxc2usq
b220b7b0|bafk4bzacicalv23ztyh3m4rhueozahnrebaozvd56ssnhevius7g3kzjgp62br3i2qnm4cvx57i7nzdtawp3tfgz2xr756ejhjg7l5gubclc3lsl
cc99eed7ec|bafk4bzacibswwdbjrvd3rr6ecfa56sazyuqfxgc26mqaiuspzb6fu4t4mxltiq33r65rhz3vmq3u6yyjv52v2g7phzm5ov2o5jjsgihzyguyhywc
04c7d959fa10|bafk4bzaciat7qfhqck6zlzcskwokjzkmh4potynarrx2xfxrlylwekxhdrcwdg2xwpyd7dhdmv3q53nytecowbvsfn3pxflgk2qj5px5d4qw54ks
5af125cb528747|bafk4bzaciay27dzo6egbmlwnmqvp3yrcyxsbsm5n2gotlkfoypqqioz7yysiomx2bkeldcyoiewzz4tf56dqsbfyxvzsramxrw6ct56knboxwtti
6122b07cc0da39b3|bafk4bzacibaedhjvqqrdhgc6zdaymcmq4t4aday4de2cylughf4fe33x64xnaltphphm53eutjrrnwl2ygbyl4tyaixm33mfomwm5tgp5yrffr4a
68b2d371c640603f22|bafk4bzacidadqjh2zowxscrnwbyuikvtwj57eo5ycuxcr224yt3an7kg5y7j644bnn26d7skhsgf6voboqqj3rnqlm2veyzm5l7gikhvigww6v6o
1c44c96abf4987f453b3|bafk4bzaciaey7llgadrjlbumfk2fu6z74afm6ev4fkqpcmriffomwqfsg3ttpjvbwjkenewpv5qgspdmwcfq6kc5vq525bbybzmfykf5lwlxbx72
b5b618f3033ebf9a0148ad|bafk4bzacidrxeaiue6yzuxgtw7r62qjyv65mifr7qejrq3mlwpdwocpacr7j6mayghrtz44abjwfk3im7qze4kjkgwt7wsbotvyjxloeqtxh5m23`
	testCases := strings.Split(testCasesString, "\n")
	for _, testCaseStr := range testCases {
		testCase := strings.Split(testCaseStr, "|")
		seed, err := hex.DecodeString(testCase[0])
		if err != nil {
			t.Error(err)
		}
		expect, err := cid.Decode(testCase[1])
		if err != nil {
			t.Error(err)
		}
		actual, err := NewMIdFromBytes(seed)
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, expect, actual)
	}

}
