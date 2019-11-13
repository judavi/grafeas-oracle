package main

import (
	"log"

	"github.com/grafeas/grafeas/go/v1beta1/server"
	grafeasStorage "github.com/grafeas/grafeas/go/v1beta1/storage"
	"github.com/judavi/grafeas-oracle/go/v1beta1/storage"
)

func main() {
	err := grafeasStorage.RegisterDefaultStorageTypeProviders()
	if err != nil {
		log.Panicf("Error when registering storage type providers, %s", err)
	}

	// register a new storage type using the key 'oracle'
	err = grafeasStorage.RegisterStorageTypeProvider("oracle", storage.OracleStorageTypeProvider)

	if err != nil {
		log.Panicf("Error when registering my new storage, %s", err)
	}

	server.StartGrafeas()
}
