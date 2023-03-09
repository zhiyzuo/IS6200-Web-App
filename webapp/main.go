package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
    "reflect"
    "encoding/json"

	"net/http"
    "html/template"

	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/gateway"
)

// Asset describes basic details of what makes up a simple asset
//Insert struct field in alphabetic order => to achieve determinism accross languages
// golang keeps the order when marshal to json but doesn't order automatically
type Asset struct {
	AppraisedValue int    `json:"AppraisedValue"`
	Color          string `json:"Color"`
	ID             string `json:"ID"`
	Owner          string `json:"Owner"`
	Size           int    `json:"Size"`
}


func main() {
	log.Println("============ application-golang starts ============")

	err := os.Setenv("DISCOVERY_AS_LOCALHOST", "true")
	if err != nil {
		log.Fatalf("Error setting DISCOVERY_AS_LOCALHOST environemnt variable: %v", err)
	}

	wallet, err := gateway.NewFileSystemWallet("wallet")
	if err != nil {
		log.Fatalf("Failed to create wallet: %v", err)
	}

	if !wallet.Exists("appUser") {
		err = populateWallet(wallet)
		if err != nil {
			log.Fatalf("Failed to populate wallet contents: %v", err)
		}
	}

	ccpPath := filepath.Join(
		"organizations",
		"peerOrganizations",
		"org1.example.com",
		"connection-org1.yaml",
	)

	gw, err := gateway.Connect(
		gateway.WithConfig(config.FromFile(filepath.Clean(ccpPath))),
		gateway.WithIdentity(wallet, "appUser"),
	)
	if err != nil {
		log.Fatalf("Failed to connect to gateway: %v", err)
	}
	defer gw.Close()

	network, err := gw.GetNetwork("mychannel")
	if err != nil {
		log.Fatalf("Failed to get network: %v", err)
	}

	contract := network.GetContract("basic")

    initLedger(contract)

	log.Println("--> Evaluate Transaction: GetAllAssets, function returns all the current assets on the ledger")
    result, err := contract.EvaluateTransaction("GetAllAssets")
	if err != nil {
		log.Fatalf("Failed to evaluate transaction: %v", err)
	}
	log.Println(string(result))

	log.Println("--> Evaluate Transaction:Read one asset")
    assetJSON, err := contract.EvaluateTransaction("ReadAsset", "asset1")
    if err != nil {
        log.Fatalf("Failed to evaluate transaction: %v\n", err)
    }
	log.Println(string(assetJSON))
	log.Println(reflect.TypeOf(assetJSON))

    var asset Asset 
    json.Unmarshal(assetJSON, &asset)
    fmt.Printf("assetid: %s, owner: %s\n", asset.ID, asset.Owner)

    runServer(contract)
}

func runServer(contract *gateway.Contract){
	http.HandleFunc("/", handler)
	http.HandleFunc("/query", queryHandler)
	http.HandleFunc("/queryAll", queryAllHandler(contract))
	http.HandleFunc("/queryResult", queryResultHandler(contract))
	http.HandleFunc("/create", createHandler)
	http.HandleFunc("/createResult", createResultHandler(contract))
	http.HandleFunc("/trade", tradeHandler)
	http.HandleFunc("/tradeResult", tradeResultHandler(contract))
    log.Fatal(http.ListenAndServe(":3000", nil))
}

func handler(w http.ResponseWriter, r *http.Request) {
    http.ServeFile(w, r, "index.html")
}

func queryHandler(w http.ResponseWriter, r *http.Request) {
    http.ServeFile(w, r, "query.html")
}

func queryResultHandler(contract *gateway.Contract) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        assetID := r.FormValue("body")
        assetJSON, err := contract.EvaluateTransaction("ReadAsset", assetID)
        if err != nil {
			http.ServeFile(w, r, "index.html")
			//http.Redirect(w, r, '/', http.StatusSeeOther)
            //log.Fatalf("Failed to evaluate transaction: %v\n", err)
        }
        var asset Asset 
		fmt.Printf("Single query: %s\n", assetJSON)
        json.Unmarshal(assetJSON, &asset)
		fmt.Printf("Single query: %s\n", asset)
        temp, _ := template.ParseFiles("queryResult.html")
        var assetList []Asset
        assetList = append(assetList, asset)
        temp.Execute(w, &assetList)
    }       
}

func queryAllHandler(contract *gateway.Contract) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        assetJSON, err := contract.EvaluateTransaction("GetAllAssets")
        if err != nil {
            log.Fatalf("Failed to evaluate transaction: %v\n", err)
			http.ServeFile(w, r, "index.html")
        }
		//fmt.Printf("GetAllAssets: %s\n", assetJSON)
        var allAssets []Asset
		//fmt.Printf("GetAllAssets: %s\n", assetJSON)
        json.Unmarshal(assetJSON, &allAssets)
		//fmt.Printf("GetAllAssets:%s\n", &allAssets)
        temp, _ := template.ParseFiles("queryResult.html")
        temp.Execute(w, &allAssets)
    }       
}

func createHandler(w http.ResponseWriter, r *http.Request) {
    http.ServeFile(w, r, "create.html")
}

func createResultHandler(contract *gateway.Contract) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        assetID := r.FormValue("aID")
        assetColor := r.FormValue("aColor")
        assetSize := r.FormValue("aSize")
        assetOwner := r.FormValue("aOwner")
        assetValue := r.FormValue("aValue")
        // read this asset
        tmpVal, err := contract.SubmitTransaction("CreateAsset", assetID, assetColor, assetSize, assetOwner, assetValue)
		fmt.Printf("Create: %s\n",  tmpVal)
        if err != nil {
			http.ServeFile(w, r, "index.html")
            //log.Fatalf("Failed to read asset: %v\n", err)
        }
        // show status on a new page
        assetJSON, err := contract.EvaluateTransaction("ReadAsset", assetID)
        var asset Asset 
        json.Unmarshal(assetJSON, &asset)
        renderTemplate(w, "createResult", &asset)
    }
}


func tradeHandler(w http.ResponseWriter, r *http.Request) {
    http.ServeFile(w, r, "trade.html")
}


func tradeResultHandler(contract *gateway.Contract) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        assetID := r.FormValue("assetID")
        assetNewOwner := r.FormValue("assetNewOwner")
        // read this asset
        assetJSON, err := contract.EvaluateTransaction("ReadAsset", assetID)
        if err != nil {
			//http.ServeFile(w, r, "index.html")
            //log.Fatalf("Failed to read asset: %v\n", err)
        }
        var asset Asset 
        json.Unmarshal(assetJSON, &asset)
        // transfer ownerhsip
        contract.SubmitTransaction("TransferAsset", assetID, assetNewOwner)
        // show status on a new page
        asset.Owner = string(assetNewOwner)
        renderTemplate(w, "tradeResult", &asset)
    }
}


func renderTemplate(w http.ResponseWriter, tmpl string, asset *Asset) {
    t, _ := template.ParseFiles(tmpl + ".html")
    t.Execute(w, asset)
}

func initLedger(contract *gateway.Contract) {
	log.Println("--> Submit Transaction: InitLedger, function creates the initial set of assets on the ledger")
	result, err := contract.SubmitTransaction("InitLedger")
	if err != nil {
		log.Fatalf("Failed to Submit transaction: %v", err)
	}
	log.Println(string(result))
}


func populateWallet(wallet *gateway.Wallet) error {
	log.Println("============ Populating wallet ============")
	credPath := filepath.Join(
		"organizations",
		"peerOrganizations",
		"org1.example.com",
		"users",
		"User1@org1.example.com",
		"msp",
	)

	certPath := filepath.Join(credPath, "signcerts", "User1@org1.example.com-cert.pem")
	// read the certificate pem
	cert, err := ioutil.ReadFile(filepath.Clean(certPath))
	if err != nil {
		return err
	}

	keyDir := filepath.Join(credPath, "keystore")
	// there's a single file in this dir containing the private key
	files, err := ioutil.ReadDir(keyDir)
	if err != nil {
		return err
	}
	if len(files) != 1 {
		return fmt.Errorf("keystore folder should have contain one file")
	}
	keyPath := filepath.Join(keyDir, files[0].Name())
	key, err := ioutil.ReadFile(filepath.Clean(keyPath))
	if err != nil {
		return err
	}

	identity := gateway.NewX509Identity("Org1MSP", string(cert), string(key))

	return wallet.Put("appUser", identity)
}
