/*
Copyright 2016 IBM

Licensed under the Apache License, Version 2.0 (the "License")
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

Licensed Materials - Property of IBM
© Copyright IBM Corp. 2016
*/
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"reflect"
	"unsafe"

	"github.com/hyperledger/fabric/core/chaincode/shim"
)

func BytesToString(b []byte) string {
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	sh := reflect.StringHeader{bh.Data, bh.Len}
	return *(*string)(unsafe.Pointer(&sh))
}

func StringToBytes(s string) []byte {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := reflect.SliceHeader{sh.Data, sh.Len, 0}
	return *(*[]byte)(unsafe.Pointer(&bh))
}

var cpPrefix = "cp:"
var coffeeAssetPrefix = "coffee:"
var accountPrefix = "acct:"
var accountsKey = "accounts"

var recentLeapYear = 2016

// SimpleChaincode example simple Chaincode implementation
type SimpleChaincode struct {
}

func generateCUSIPSuffix(issueDate string, days int) (string, error) {

	t, err := msToTime(issueDate)
	if err != nil {
		return "", err
	}

	maturityDate := t.AddDate(0, 0, days)
	month := int(maturityDate.Month())
	day := maturityDate.Day()

	suffix := seventhDigit[month] + eigthDigit[day]
	return suffix, nil

}

const (
	millisPerSecond     = int64(time.Second / time.Millisecond)
	nanosPerMillisecond = int64(time.Millisecond / time.Nanosecond)
)

func msToTime(ms string) (time.Time, error) {
	msInt, err := strconv.ParseInt(ms, 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(msInt/millisPerSecond,
		(msInt%millisPerSecond)*nanosPerMillisecond), nil
}

type Owner struct {
	Company  string `json:"company"`
	Quantity int    `json:"quantity"`
}

type CP struct {
	CUSIP     string  `json:"cusip"`
	Ticker    string  `json:"ticker"`
	Par       float64 `json:"par"`
	Qty       int     `json:"qty"`
	Discount  float64 `json:"discount"`
	Maturity  int     `json:"maturity"`
	Owners    []Owner `json:"owner"`
	Issuer    string  `json:"issuer"`
	IssueDate string  `json:"issueDate"`
}

type CoffeeAsset struct {
	UUID        string  `json:"uuid"`
	Amount      int     `json:"amount"`
	Owners      []Owner `json:"owner"`
	Grower      string  `json:"grower"`
	HarvestDate int     `json:"harvestDate"`
}

type Account struct {
	ID          string   `json:"id"`
	Prefix      string   `json:"prefix"`
	CashBalance float64  `json:"cashBalance"`
	AssetsIds   []string `json:"assetIds"`
}

type Transaction struct {
	CUSIP       string  `json:"cusip"`
	FromCompany string  `json:"fromCompany"`
	ToCompany   string  `json:"toCompany"`
	Quantity    int     `json:"quantity"`
	Discount    float64 `json:"discount"`
}

func (t *SimpleChaincode) Init(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	// Initialize the collection of commercial paper keys
	fmt.Println("Initializing paper keys collection")
	var blank []string
	blankBytes, _ := json.Marshal(&blank)
	err := stub.PutState("PaperKeys", blankBytes)
	if err != nil {
		fmt.Println("Failed to initialize paper key collection")
	}

	fmt.Println("Initialization complete")
	return nil, nil
}

func (t *SimpleChaincode) createAccounts(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {

	//  				0
	// "number of accounts to create"
	var err error
	numAccounts, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Println("error creating accounts with input")
		return nil, errors.New("createAccounts accepts a single integer argument")
	}
	//create a bunch of accounts
	var account Account
	counter := 1
	for counter <= numAccounts {
		var prefix string
		suffix := "000A"
		if counter < 10 {
			prefix = strconv.Itoa(counter) + "0" + suffix
		} else {
			prefix = strconv.Itoa(counter) + suffix
		}
		var assetIds []string
		account = Account{ID: "company" + strconv.Itoa(counter), Prefix: prefix, CashBalance: 10000000.0, AssetsIds: assetIds}
		accountBytes, err := json.Marshal(&account)
		if err != nil {
			fmt.Println("error creating account" + account.ID)
			return nil, errors.New("Error creating account " + account.ID)
		}
		err = stub.PutState(accountPrefix+account.ID, accountBytes)
		counter++
		fmt.Println("created account" + accountPrefix + account.ID)
	}

	fmt.Println("Accounts created")
	return nil, nil

}

func (t *SimpleChaincode) createAccount(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	// Obtain the username to associate with the account
	if len(args) != 1 {
		fmt.Println("Error obtaining username")
		return nil, errors.New("createAccount accepts a single username argument")
	}
	username := args[0]

	// Build an account object for the user
	var assetIds []string
	suffix := "000A"
	prefix := username + suffix
	var account = Account{ID: username, Prefix: prefix, CashBalance: 10000000.0, AssetsIds: assetIds}
	accountBytes, err := json.Marshal(&account)
	if err != nil {
		fmt.Println("error creating account" + account.ID)
		return nil, errors.New("Error creating account " + account.ID)
	}

	fmt.Println("Attempting to get state of any existing account for " + account.ID)
	existingBytes, err := stub.GetState(accountPrefix + account.ID)
	if err == nil {

		var company Account
		err = json.Unmarshal(existingBytes, &company)
		if err != nil {
			fmt.Println("Error unmarshalling account " + account.ID + "\n--->: " + err.Error())

			if strings.Contains(err.Error(), "unexpected end") {
				fmt.Println("No data means existing account found for " + account.ID + ", initializing account.")
				err = stub.PutState(accountPrefix+account.ID, accountBytes)

				if err == nil {
					fmt.Println("created account" + accountPrefix + account.ID)
					return nil, nil
				} else {
					fmt.Println("failed to create initialize account for " + account.ID)
					return nil, errors.New("failed to initialize an account for " + account.ID + " => " + err.Error())
				}
			} else {
				return nil, errors.New("Error unmarshalling existing account " + account.ID)
			}
		} else {
			fmt.Println("Account already exists for " + account.ID + " " + company.ID)
			return nil, errors.New("Can't reinitialize existing user " + account.ID)
		}
	} else {

		fmt.Println("No existing account found for " + account.ID + ", initializing account.")
		err = stub.PutState(accountPrefix+account.ID, accountBytes)

		if err == nil {
			fmt.Println("created account" + accountPrefix + account.ID)
			return nil, nil
		} else {
			fmt.Println("failed to create initialize account for " + account.ID)
			return nil, errors.New("failed to initialize an account for " + account.ID + " => " + err.Error())
		}

	}

}

func (t *SimpleChaincode) testCreateCoffeeAsset(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {

	var username = "farmer1"
	var amount = 10
	var assetIds []string
	var suffix = "000A"
	var prefix = username + suffix
	var coffeeUUID = "testAsset"

	var account = Account{ID: username, Prefix: prefix, CashBalance: 10000000.0, AssetsIds: assetIds}
	var owners = []Owner{{
		Company:  username,
		Quantity: amount}}

	var coffeeAsset = CoffeeAsset{UUID: coffeeUUID, Amount: amount, Owners: owners, Grower: username, HarvestDate: 1456161763790}

	var newAccountArgs [1]string
	var newCoffeeAssetArgs [1]string

	accountBytes, err := json.Marshal(&account)
	if err != nil {
		fmt.Println("error marshalling account")
		return nil, errors.New("Error marshalling account " + account.ID)
	}
	coffeeAssetBytes, err := json.Marshal(&coffeeAsset)
	if err != nil {
		fmt.Println("error marshalling coffee asset")
		return nil, errors.New("error marshalling coffee asset " + coffeeAsset.Grower)
	}

	// TODO this should be the name of the farmer
	newAccountArgs[0] = fmt.Sprintf("%s", accountBytes)
	newAccountArgsArray := newAccountArgs[:]

	t.createAccount(stub, newAccountArgsArray)

	// now create the coffee asset
	// newCoffeeAssetArgs[0] = fmt.Sprintf("%s", coffeeAssetBytes)
	// newCoffeeAssetArgsArray := newCoffeeAssetArgs[:]
	// return t.createCoffeeAsset(stub, newCoffeeAssetArgsArray)

}

//**********
/*
type CoffeeAsset struct {
	UUID        string  `json:"uuid"`
	Amount      int     `json:"amount"`
	Owners      []Owner `json:"owner"`
	Grower      Owner   `json:"grower"`
	HarvestDate string  `json:"harvestDate"`
}
*/
func (t *SimpleChaincode) createCoffeeAsset(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {

	//need one arg
	if len(args) != 1 {
		fmt.Println("error invalid arguments")
		return nil, errors.New("Incorrect number of arguments. Expecting CoffeeAsset record")
	}

	var coffeeAsset CoffeeAsset
	//var cp CP
	var err error
	var account Account

	fmt.Println("Unmarshalling Coffee Asset")
	err = json.Unmarshal([]byte(args[0]), &coffeeAsset)
	if err != nil {
		fmt.Println("error invalid coffee asset")
		return nil, errors.New("Invalid coffee asset")
	}

	//generate the CUSIP
	//get account prefix
	fmt.Println("Getting state of - " + accountPrefix + coffeeAsset.Grower)
	accountBytes, err := stub.GetState(accountPrefix + coffeeAsset.Grower)
	if err != nil {
		fmt.Println("Error Getting state of - " + accountPrefix + coffeeAsset.Grower)
		return nil, errors.New("Error retrieving account " + coffeeAsset.Grower)
	}
	err = json.Unmarshal(accountBytes, &account)
	if err != nil {
		fmt.Println("Error Unmarshalling accountBytes")
		return nil, errors.New("Error retrieving account " + coffeeAsset.Grower)
	}

	account.AssetsIds = append(account.AssetsIds, coffeeAsset.UUID)

	// Set the Grower to be the owner of all quantity
	var owner Owner
	owner.Company = coffeeAsset.Grower
	owner.Quantity = coffeeAsset.Amount

	coffeeAsset.Owners = append(coffeeAsset.Owners, owner)

	// suffix, err := generateCUSIPSuffix(cp.IssueDate, cp.Maturity)
	var suffix = "1234" //strconv.Itoa(time.Now().UnixNano())
	// if err != nil {
	// 	fmt.Println("Error generating cusip")
	// 	return nil, errors.New("Error generating CUSIP")
	// }

	fmt.Println("Marshalling coffee asset bytes")
	if coffeeAsset.UUID == "" {
		coffeeAsset.UUID = suffix
	}

	fmt.Println("Getting State of Coffee Asset " + coffeeAsset.UUID)
	coffeeAssetRxBytes, err := stub.GetState(coffeeAssetPrefix + coffeeAsset.UUID)
	if coffeeAssetRxBytes == nil {
		fmt.Println("Coffee Asset does not exist, creating it")
		coffeeAssetBytes, err := json.Marshal(&coffeeAsset)
		if err != nil {
			fmt.Println("Error marshalling coffee asset")
			return nil, errors.New("Error creating coffee asset")
		}
		err = stub.PutState(coffeeAssetPrefix+coffeeAsset.UUID, coffeeAssetBytes)
		if err != nil {
			fmt.Println("Error creating coffee asset")
			return nil, errors.New("Error creating coffee asset")
		}

		fmt.Println("Marshalling account bytes to write")
		accountBytesToWrite, err := json.Marshal(&account)
		if err != nil {
			fmt.Println("Error marshalling account")
			return nil, errors.New("Error creating coffee asset")
		}
		err = stub.PutState(accountPrefix+coffeeAsset.Grower, accountBytesToWrite)
		if err != nil {
			fmt.Println("Error putting state on accountBytesToWrite")
			return nil, errors.New("Error creating coffee asset")
		}

		// Update the coffee asset keys by adding the new key
		fmt.Println("Getting Coffee Asset Keys")
		keysBytes, err := stub.GetState("CoffeeAssetKeys")
		if err != nil {
			fmt.Println("Error retrieving coffee asset keys")
			return nil, errors.New("Error retrieving coffee asset keys")
		}
		var keys []string
		err = json.Unmarshal(keysBytes, &keys)
		if err != nil {
			fmt.Println("Error unmarshel keys")
			return nil, errors.New("Error unmarshalling coffee asset keys ")
		}

		fmt.Println("Appending the new key to Coffee Asset Keys")
		foundKey := false
		for _, key := range keys {
			if key == coffeeAssetPrefix+coffeeAsset.UUID {
				foundKey = true
			}
		}
		if foundKey == false {
			keys = append(keys, coffeeAssetPrefix+coffeeAsset.UUID)
			keysBytesToWrite, err := json.Marshal(&keys)
			if err != nil {
				fmt.Println("Error marshalling keys")
				return nil, errors.New("Error marshalling the keys")
			}
			fmt.Println("Put state on CoffeeAssetKeys")
			err = stub.PutState("CoffeeAssetKeys", keysBytesToWrite)
			if err != nil {
				fmt.Println("Error writting keys back")
				return nil, errors.New("Error writing the keys back")
			}
		}

		fmt.Println("Create Coffee Asset %+v\n", coffeeAsset)
		return nil, nil
	} else {
		fmt.Println("Coffee Asset already exists, update it")

		var coffeeAssetRx CoffeeAsset
		fmt.Println("Unmarshalling coffee asset " + coffeeAsset.UUID)
		err = json.Unmarshal(coffeeAssetRxBytes, &coffeeAssetRx)
		if err != nil {
			fmt.Println("Error unmarshalling coffee Asset" + coffeeAsset.UUID)
			return nil, errors.New("Error unmarshalling cp " + coffeeAsset.UUID)
		}

		coffeeAsset.Amount = coffeeAssetRx.Amount + coffeeAsset.Amount

		for key, val := range coffeeAssetRx.Owners {
			if val.Company == coffeeAsset.Grower {
				coffeeAssetRx.Owners[key].Quantity += coffeeAsset.Amount
				break
			}
		}

		coffeeAssetWriteBytes, err := json.Marshal(&coffeeAssetRx)
		if err != nil {
			fmt.Println("Error marshalling coffee asset")
			return nil, errors.New("Error creating coffee asset")
		}
		err = stub.PutState(coffeeAssetPrefix+coffeeAsset.UUID, coffeeAssetWriteBytes)
		if err != nil {
			fmt.Println("Error creating new coffee asset")
			return nil, errors.New("Error creating new coffee asset")
		}

		fmt.Println("Updated coffee asset %+v\n", coffeeAssetRx)
		return nil, nil
	}
}

func GetAllCoffeeAssets(stub shim.ChaincodeStubInterface) ([]CoffeeAsset, error) {

	var allcoffeeAssets []CoffeeAsset

	// Get list of all the keys
	keysBytes, err := stub.GetState("CoffeeAssetKeys")
	if err != nil {
		fmt.Println("Error retrieving coffee asset keys")
		return nil, errors.New("Error retrieving coffee asset keys")
	}
	var keys []string
	err = json.Unmarshal(keysBytes, &keys)
	if err != nil {
		fmt.Println("Error unmarshalling coffee asset keys")
		return nil, errors.New("Error unmarshalling coffee asset keys")
	}

	// Get all the coffee assets
	for _, value := range keys {
		coffeeAssetBytes, err := stub.GetState(value)

		var coffeeAsset CoffeeAsset
		err = json.Unmarshal(coffeeAssetBytes, &coffeeAsset)
		if err != nil {
			fmt.Println("Error retrieving coffee asset " + value)
			return nil, errors.New("Error retrieving coffee asset " + value)
		}

		fmt.Println("Appending coffee Asset " + value)
		allcoffeeAssets = append(allcoffeeAssets, coffeeAsset)
	}

	return allcoffeeAssets, nil
}

func GetCoffeeAsset(coffeeAssetId string, stub shim.ChaincodeStubInterface) (CoffeeAsset, error) {
	var coffeeAsset CoffeeAsset

	coffeeAssetBytes, err := stub.GetState(coffeeAssetId)
	if err != nil {
		fmt.Println("Error retrieving coffee asset " + coffeeAssetId)
		return coffeeAsset, errors.New("Error retrieving cp " + coffeeAssetId)
	}

	err = json.Unmarshal(coffeeAssetBytes, &coffeeAsset)
	if err != nil {
		fmt.Println("Error unmarshalling coffee asset " + coffeeAssetId)
		return coffeeAsset, errors.New("Error unmarshalling coffee asset " + coffeeAssetId)
	}

	return coffeeAsset, nil
}

//*******

func (t *SimpleChaincode) issueCommercialPaper(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {

	/*		0
		json
	  	{
			"ticker":  "string",
			"par": 0.00,
			"qty": 10,
			"discount": 7.5,
			"maturity": 30,
			"owners": [ // This one is not required
				{
					"company": "company1",
					"quantity": 5
				},
				{
					"company": "company3",
					"quantity": 3
				},
				{
					"company": "company4",
					"quantity": 2
				}
			],
			"issuer":"company2",
			"issueDate":"1456161763790"  (current time in milliseconds as a string)

		}
	*/

	//need one arg
	if len(args) != 1 {
		fmt.Println("error invalid arguments")
		return nil, errors.New("Incorrect number of arguments. Expecting commercial paper record")
	}

	var cp CP
	var err error
	var account Account

	fmt.Println("Unmarshalling CP")
	err = json.Unmarshal([]byte(args[0]), &cp)
	if err != nil {
		fmt.Println("error invalid paper issue")
		return nil, errors.New("Invalid commercial paper issue")
	}

	//generate the CUSIP
	//get account prefix
	fmt.Println("Getting state of - " + accountPrefix + cp.Issuer)
	accountBytes, err := stub.GetState(accountPrefix + cp.Issuer)
	if err != nil {
		fmt.Println("Error Getting state of - " + accountPrefix + cp.Issuer)
		return nil, errors.New("Error retrieving account " + cp.Issuer)
	}
	err = json.Unmarshal(accountBytes, &account)
	if err != nil {
		fmt.Println("Error Unmarshalling accountBytes")
		return nil, errors.New("Error retrieving account " + cp.Issuer)
	}

	account.AssetsIds = append(account.AssetsIds, cp.CUSIP)

	// Set the issuer to be the owner of all quantity
	var owner Owner
	owner.Company = cp.Issuer
	owner.Quantity = cp.Qty

	cp.Owners = append(cp.Owners, owner)

	suffix, err := generateCUSIPSuffix(cp.IssueDate, cp.Maturity)
	if err != nil {
		fmt.Println("Error generating cusip")
		return nil, errors.New("Error generating CUSIP")
	}

	fmt.Println("Marshalling CP bytes")
	cp.CUSIP = account.Prefix + suffix

	fmt.Println("Getting State on CP " + cp.CUSIP)
	cpRxBytes, err := stub.GetState(cpPrefix + cp.CUSIP)
	if cpRxBytes == nil {
		fmt.Println("CUSIP does not exist, creating it")
		cpBytes, err := json.Marshal(&cp)
		if err != nil {
			fmt.Println("Error marshalling cp")
			return nil, errors.New("Error issuing commercial paper")
		}
		err = stub.PutState(cpPrefix+cp.CUSIP, cpBytes)
		if err != nil {
			fmt.Println("Error issuing paper")
			return nil, errors.New("Error issuing commercial paper")
		}

		fmt.Println("Marshalling account bytes to write")
		accountBytesToWrite, err := json.Marshal(&account)
		if err != nil {
			fmt.Println("Error marshalling account")
			return nil, errors.New("Error issuing commercial paper")
		}
		err = stub.PutState(accountPrefix+cp.Issuer, accountBytesToWrite)
		if err != nil {
			fmt.Println("Error putting state on accountBytesToWrite")
			return nil, errors.New("Error issuing commercial paper")
		}

		// Update the paper keys by adding the new key
		fmt.Println("Getting Paper Keys")
		keysBytes, err := stub.GetState("PaperKeys")
		if err != nil {
			fmt.Println("Error retrieving paper keys")
			return nil, errors.New("Error retrieving paper keys")
		}
		var keys []string
		err = json.Unmarshal(keysBytes, &keys)
		if err != nil {
			fmt.Println("Error unmarshel keys")
			return nil, errors.New("Error unmarshalling paper keys ")
		}

		fmt.Println("Appending the new key to Paper Keys")
		foundKey := false
		for _, key := range keys {
			if key == cpPrefix+cp.CUSIP {
				foundKey = true
			}
		}
		if foundKey == false {
			keys = append(keys, cpPrefix+cp.CUSIP)
			keysBytesToWrite, err := json.Marshal(&keys)
			if err != nil {
				fmt.Println("Error marshalling keys")
				return nil, errors.New("Error marshalling the keys")
			}
			fmt.Println("Put state on PaperKeys")
			err = stub.PutState("PaperKeys", keysBytesToWrite)
			if err != nil {
				fmt.Println("Error writting keys back")
				return nil, errors.New("Error writing the keys back")
			}
		}

		fmt.Println("Issue commercial paper %+v\n", cp)
		return nil, nil
	} else {
		fmt.Println("CUSIP exists")

		var cprx CP
		fmt.Println("Unmarshalling CP " + cp.CUSIP)
		err = json.Unmarshal(cpRxBytes, &cprx)
		if err != nil {
			fmt.Println("Error unmarshalling cp " + cp.CUSIP)
			return nil, errors.New("Error unmarshalling cp " + cp.CUSIP)
		}

		cprx.Qty = cprx.Qty + cp.Qty

		for key, val := range cprx.Owners {
			if val.Company == cp.Issuer {
				cprx.Owners[key].Quantity += cp.Qty
				break
			}
		}

		cpWriteBytes, err := json.Marshal(&cprx)
		if err != nil {
			fmt.Println("Error marshalling cp")
			return nil, errors.New("Error issuing commercial paper")
		}
		err = stub.PutState(cpPrefix+cp.CUSIP, cpWriteBytes)
		if err != nil {
			fmt.Println("Error issuing paper")
			return nil, errors.New("Error issuing commercial paper")
		}

		fmt.Println("Updated commercial paper %+v\n", cprx)
		return nil, nil
	}
}

func GetAllCPs(stub shim.ChaincodeStubInterface) ([]CP, error) {

	var allCPs []CP

	// Get list of all the keys
	keysBytes, err := stub.GetState("PaperKeys")
	if err != nil {
		fmt.Println("Error retrieving paper keys")
		return nil, errors.New("Error retrieving paper keys")
	}
	var keys []string
	err = json.Unmarshal(keysBytes, &keys)
	if err != nil {
		fmt.Println("Error unmarshalling paper keys")
		return nil, errors.New("Error unmarshalling paper keys")
	}

	// Get all the cps
	for _, value := range keys {
		cpBytes, err := stub.GetState(value)

		var cp CP
		err = json.Unmarshal(cpBytes, &cp)
		if err != nil {
			fmt.Println("Error retrieving cp " + value)
			return nil, errors.New("Error retrieving cp " + value)
		}

		fmt.Println("Appending CP" + value)
		allCPs = append(allCPs, cp)
	}

	return allCPs, nil
}

func GetCP(cpid string, stub shim.ChaincodeStubInterface) (CP, error) {
	var cp CP

	cpBytes, err := stub.GetState(cpid)
	if err != nil {
		fmt.Println("Error retrieving cp " + cpid)
		return cp, errors.New("Error retrieving cp " + cpid)
	}

	err = json.Unmarshal(cpBytes, &cp)
	if err != nil {
		fmt.Println("Error unmarshalling cp " + cpid)
		return cp, errors.New("Error unmarshalling cp " + cpid)
	}

	return cp, nil
}

func GetCompany(companyID string, stub shim.ChaincodeStubInterface) (Account, error) {
	var company Account
	companyBytes, err := stub.GetState(accountPrefix + companyID)
	if err != nil {
		fmt.Println("Account not found " + companyID)
		return company, errors.New("Account not found " + companyID)
	}

	err = json.Unmarshal(companyBytes, &company)
	if err != nil {
		fmt.Println("Error unmarshalling account " + companyID + "\n err:" + err.Error())
		return company, errors.New("Error unmarshalling account " + companyID)
	}

	return company, nil
}

/*
*	Transfer coffee asset from one owner to another
 */
func (t *SimpleChaincode) transferCoffeeAsset(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	/*		0
		json
	  	{
			  "CUSIP": "",
			  "fromCompany":"",
			  "toCompany":"",
			  "quantity": 1
		}
	*/
	//need one arg
	if len(args) != 1 {
		return nil, errors.New("Incorrect number of arguments. Expecting transaction record")
	}

	var tr Transaction

	fmt.Println("Unmarshalling Transaction")
	err := json.Unmarshal([]byte(args[0]), &tr)
	if err != nil {
		fmt.Println("Error Unmarshalling Transaction")
		return nil, errors.New("Invalid coffee asset")
	}

	fmt.Println("Getting State on Coffee Asset " + tr.CUSIP)
	coffeeAssetBytes, err := stub.GetState(coffeeAssetPrefix + tr.CUSIP)
	if err != nil {
		fmt.Println("CUSIP not found")
		return nil, errors.New("CUSIP not found " + tr.CUSIP)
	}

	var coffeeAsset CoffeeAsset
	fmt.Println("Unmarshalling Coffee Asset " + tr.CUSIP)
	err = json.Unmarshal(coffeeAssetBytes, &coffeeAsset)
	if err != nil {
		fmt.Println("Error unmarshalling coffeeAsset " + tr.CUSIP)
		return nil, errors.New("Error unmarshalling coffeeAsset " + tr.CUSIP)
	}

	var fromCompany Account
	fmt.Println("Getting State on fromCompany " + tr.FromCompany)
	fromCompanyBytes, err := stub.GetState(accountPrefix + tr.FromCompany)
	if err != nil {
		fmt.Println("Account not found " + tr.FromCompany)
		return nil, errors.New("Account not found " + tr.FromCompany)
	}

	fmt.Println("Unmarshalling FromCompany ")
	err = json.Unmarshal(fromCompanyBytes, &fromCompany)
	if err != nil {
		fmt.Println("Error unmarshalling account " + tr.FromCompany)
		return nil, errors.New("Error unmarshalling account " + tr.FromCompany)
	}

	var toCompany Account
	fmt.Println("Getting State on ToCompany " + tr.ToCompany)
	toCompanyBytes, err := stub.GetState(accountPrefix + tr.ToCompany)
	if err != nil {
		fmt.Println("Account not found " + tr.ToCompany)
		return nil, errors.New("Account not found " + tr.ToCompany)
	}

	fmt.Println("Unmarshalling tocompany")
	err = json.Unmarshal(toCompanyBytes, &toCompany)
	if err != nil {
		fmt.Println("Error unmarshalling account " + tr.ToCompany)
		return nil, errors.New("Error unmarshalling account " + tr.ToCompany)
	}

	// Check for all the possible errors
	ownerFound := false
	quantity := 0
	for _, owner := range coffeeAsset.Owners {
		if owner.Company == tr.FromCompany {
			ownerFound = true
			quantity = owner.Quantity
		}
	}

	// If fromCompany doesn't own this coffee asset
	if ownerFound == false {
		fmt.Println("The company " + tr.FromCompany + "doesn't own any of this coffee asset")
		return nil, errors.New("The company " + tr.FromCompany + "doesn't own any of this coffee asset")
	} else {
		fmt.Println("The FromCompany does own this coffee asset")
	}

	// If fromCompany doesn't own enough quantity of this coffee asset
	if quantity < tr.Quantity {
		fmt.Println("The company " + tr.FromCompany + "doesn't own enough of this coffee asset")
		return nil, errors.New("The company " + tr.FromCompany + "doesn't own enough of this coffee asset")
	} else {
		fmt.Println("The FromCompany owns enough of this coffee asset")
	}

	amountToBeTransferred := float64(tr.Quantity) // * cp.Par
	//amountToBeTransferred -= (amountToBeTransferred) * (cp.Discount / 100.0) * (float64(cp.Maturity) / 360.0)

	// If toCompany doesn't have enough cash to buy the papers
	if toCompany.CashBalance < amountToBeTransferred {
		fmt.Println("The company " + tr.ToCompany + "doesn't have enough cash to purchase the coffee asset")
		return nil, errors.New("The company " + tr.ToCompany + "doesn't have enough cash to purchase the coffee asset")
	} else {
		fmt.Println("The ToCompany has enough money to be transferred for this coffee asset")
	}

	toCompany.CashBalance -= amountToBeTransferred
	fromCompany.CashBalance += amountToBeTransferred

	toOwnerFound := false
	for key, owner := range coffeeAsset.Owners {
		if owner.Company == tr.FromCompany {
			fmt.Println("Reducing Quantity from the FromCompany")
			coffeeAsset.Owners[key].Quantity -= tr.Quantity
			//			owner.Quantity -= tr.Quantity
		}
		if owner.Company == tr.ToCompany {
			fmt.Println("Increasing Quantity from the ToCompany")
			toOwnerFound = true
			coffeeAsset.Owners[key].Quantity += tr.Quantity
			//			owner.Quantity += tr.Quantity
		}
	}

	if toOwnerFound == false {
		var newOwner Owner
		fmt.Println("As ToOwner was not found, appending the owner to the Coffee Asset")
		newOwner.Quantity = tr.Quantity
		newOwner.Company = tr.ToCompany
		coffeeAsset.Owners = append(coffeeAsset.Owners, newOwner)
	}

	fromCompany.AssetsIds = append(fromCompany.AssetsIds, tr.CUSIP)

	// Write everything back
	// To Company
	toCompanyBytesToWrite, err := json.Marshal(&toCompany)
	if err != nil {
		fmt.Println("Error marshalling the toCompany")
		return nil, errors.New("Error marshalling the toCompany")
	}
	fmt.Println("Put state on toCompany")
	err = stub.PutState(accountPrefix+tr.ToCompany, toCompanyBytesToWrite)
	if err != nil {
		fmt.Println("Error writing the toCompany back")
		return nil, errors.New("Error writing the toCompany back")
	}

	// From company
	fromCompanyBytesToWrite, err := json.Marshal(&fromCompany)
	if err != nil {
		fmt.Println("Error marshalling the fromCompany")
		return nil, errors.New("Error marshalling the fromCompany")
	}
	fmt.Println("Put state on fromCompany")
	err = stub.PutState(accountPrefix+tr.FromCompany, fromCompanyBytesToWrite)
	if err != nil {
		fmt.Println("Error writing the fromCompany back")
		return nil, errors.New("Error writing the fromCompany back")
	}

	// coffeeAsset
	coffeeAssetBytesToWrite, err := json.Marshal(&coffeeAsset)
	if err != nil {
		fmt.Println("Error marshalling the coffeeAsset")
		return nil, errors.New("Error marshalling the coffeeAsset")
	}
	fmt.Println("Put state on coffeeAsset")
	err = stub.PutState(coffeeAssetPrefix+tr.CUSIP, coffeeAssetBytesToWrite)
	if err != nil {
		fmt.Println("Error writing the coffeeAsset back")
		return nil, errors.New("Error writing the coffeeAsset back")
	}

	fmt.Println("Successfully completed Invoke")
	return nil, nil
}

// Still working on this one
func (t *SimpleChaincode) transferPaper(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	/*		0
		json
	  	{
			  "CUSIP": "",
			  "fromCompany":"",
			  "toCompany":"",
			  "quantity": 1
		}
	*/
	//need one arg
	if len(args) != 1 {
		return nil, errors.New("Incorrect number of arguments. Expecting commercial paper record")
	}

	var tr Transaction

	fmt.Println("Unmarshalling Transaction")
	err := json.Unmarshal([]byte(args[0]), &tr)
	if err != nil {
		fmt.Println("Error Unmarshalling Transaction")
		return nil, errors.New("Invalid commercial paper issue")
	}

	fmt.Println("Getting State on CP " + tr.CUSIP)
	cpBytes, err := stub.GetState(cpPrefix + tr.CUSIP)
	if err != nil {
		fmt.Println("CUSIP not found")
		return nil, errors.New("CUSIP not found " + tr.CUSIP)
	}

	var cp CP
	fmt.Println("Unmarshalling CP " + tr.CUSIP)
	err = json.Unmarshal(cpBytes, &cp)
	if err != nil {
		fmt.Println("Error unmarshalling cp " + tr.CUSIP)
		return nil, errors.New("Error unmarshalling cp " + tr.CUSIP)
	}

	var fromCompany Account
	fmt.Println("Getting State on fromCompany " + tr.FromCompany)
	fromCompanyBytes, err := stub.GetState(accountPrefix + tr.FromCompany)
	if err != nil {
		fmt.Println("Account not found " + tr.FromCompany)
		return nil, errors.New("Account not found " + tr.FromCompany)
	}

	fmt.Println("Unmarshalling FromCompany ")
	err = json.Unmarshal(fromCompanyBytes, &fromCompany)
	if err != nil {
		fmt.Println("Error unmarshalling account " + tr.FromCompany)
		return nil, errors.New("Error unmarshalling account " + tr.FromCompany)
	}

	var toCompany Account
	fmt.Println("Getting State on ToCompany " + tr.ToCompany)
	toCompanyBytes, err := stub.GetState(accountPrefix + tr.ToCompany)
	if err != nil {
		fmt.Println("Account not found " + tr.ToCompany)
		return nil, errors.New("Account not found " + tr.ToCompany)
	}

	fmt.Println("Unmarshalling tocompany")
	err = json.Unmarshal(toCompanyBytes, &toCompany)
	if err != nil {
		fmt.Println("Error unmarshalling account " + tr.ToCompany)
		return nil, errors.New("Error unmarshalling account " + tr.ToCompany)
	}

	// Check for all the possible errors
	ownerFound := false
	quantity := 0
	for _, owner := range cp.Owners {
		if owner.Company == tr.FromCompany {
			ownerFound = true
			quantity = owner.Quantity
		}
	}

	// If fromCompany doesn't own this paper
	if ownerFound == false {
		fmt.Println("The company " + tr.FromCompany + "doesn't own any of this paper")
		return nil, errors.New("The company " + tr.FromCompany + "doesn't own any of this paper")
	} else {
		fmt.Println("The FromCompany does own this paper")
	}

	// If fromCompany doesn't own enough quantity of this paper
	if quantity < tr.Quantity {
		fmt.Println("The company " + tr.FromCompany + "doesn't own enough of this paper")
		return nil, errors.New("The company " + tr.FromCompany + "doesn't own enough of this paper")
	} else {
		fmt.Println("The FromCompany owns enough of this paper")
	}

	amountToBeTransferred := float64(tr.Quantity) * cp.Par
	amountToBeTransferred -= (amountToBeTransferred) * (cp.Discount / 100.0) * (float64(cp.Maturity) / 360.0)

	// If toCompany doesn't have enough cash to buy the papers
	if toCompany.CashBalance < amountToBeTransferred {
		fmt.Println("The company " + tr.ToCompany + "doesn't have enough cash to purchase the papers")
		return nil, errors.New("The company " + tr.ToCompany + "doesn't have enough cash to purchase the papers")
	} else {
		fmt.Println("The ToCompany has enough money to be transferred for this paper")
	}

	toCompany.CashBalance -= amountToBeTransferred
	fromCompany.CashBalance += amountToBeTransferred

	toOwnerFound := false
	for key, owner := range cp.Owners {
		if owner.Company == tr.FromCompany {
			fmt.Println("Reducing Quantity from the FromCompany")
			cp.Owners[key].Quantity -= tr.Quantity
			//			owner.Quantity -= tr.Quantity
		}
		if owner.Company == tr.ToCompany {
			fmt.Println("Increasing Quantity from the ToCompany")
			toOwnerFound = true
			cp.Owners[key].Quantity += tr.Quantity
			//			owner.Quantity += tr.Quantity
		}
	}

	if toOwnerFound == false {
		var newOwner Owner
		fmt.Println("As ToOwner was not found, appending the owner to the CP")
		newOwner.Quantity = tr.Quantity
		newOwner.Company = tr.ToCompany
		cp.Owners = append(cp.Owners, newOwner)
	}

	fromCompany.AssetsIds = append(fromCompany.AssetsIds, tr.CUSIP)

	// Write everything back
	// To Company
	toCompanyBytesToWrite, err := json.Marshal(&toCompany)
	if err != nil {
		fmt.Println("Error marshalling the toCompany")
		return nil, errors.New("Error marshalling the toCompany")
	}
	fmt.Println("Put state on toCompany")
	err = stub.PutState(accountPrefix+tr.ToCompany, toCompanyBytesToWrite)
	if err != nil {
		fmt.Println("Error writing the toCompany back")
		return nil, errors.New("Error writing the toCompany back")
	}

	// From company
	fromCompanyBytesToWrite, err := json.Marshal(&fromCompany)
	if err != nil {
		fmt.Println("Error marshalling the fromCompany")
		return nil, errors.New("Error marshalling the fromCompany")
	}
	fmt.Println("Put state on fromCompany")
	err = stub.PutState(accountPrefix+tr.FromCompany, fromCompanyBytesToWrite)
	if err != nil {
		fmt.Println("Error writing the fromCompany back")
		return nil, errors.New("Error writing the fromCompany back")
	}

	// cp
	cpBytesToWrite, err := json.Marshal(&cp)
	if err != nil {
		fmt.Println("Error marshalling the cp")
		return nil, errors.New("Error marshalling the cp")
	}
	fmt.Println("Put state on CP")
	err = stub.PutState(cpPrefix+tr.CUSIP, cpBytesToWrite)
	if err != nil {
		fmt.Println("Error writing the cp back")
		return nil, errors.New("Error writing the cp back")
	}

	fmt.Println("Successfully completed Invoke")
	return nil, nil
}

func (t *SimpleChaincode) Query(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	//need one arg
	if len(args) < 1 {
		return nil, errors.New("Incorrect number of arguments. Expecting ......")
	}
	fmt.Println("Starting query operation with " + args[0])

	if args[0] == "GetAllCoffeeAssets" {
		fmt.Println("Getting all coffee assets")
		allcoffeeAssets, err := GetAllCoffeeAssets(stub)
		if err != nil {
			fmt.Println("Error from GetAllCoffeeAssets")
			return nil, err
		} else {
			allcoffeeAssetBytes, err1 := json.Marshal(&allcoffeeAssets)
			if err1 != nil {
				fmt.Println("Error marshalling allcoffeeAssets")
				return nil, err1
			}
			fmt.Println("All success, returning allcoffeeAssets")
			return allcoffeeAssetBytes, nil
		}
	} else if args[0] == "GetCoffeeAsset" {
		fmt.Println("Getting particular coffee asset")
		coffeeAsset, err := GetCoffeeAsset(args[1], stub)
		if err != nil {
			fmt.Println("Error Getting particular coffeeAsset")
			return nil, err
		} else {
			coffeeAssetBytes, err1 := json.Marshal(&coffeeAsset)
			if err1 != nil {
				fmt.Println("Error marshalling the coffeeAsset")
				return nil, err1
			}
			fmt.Println("All success, returning the coffeeAsset")
			return coffeeAssetBytes, nil
		}
	} else if args[0] == "GetAllCPs" {
		fmt.Println("Getting all CPs")
		allCPs, err := GetAllCPs(stub)
		if err != nil {
			fmt.Println("Error from getallcps")
			return nil, err
		} else {
			allCPsBytes, err1 := json.Marshal(&allCPs)
			if err1 != nil {
				fmt.Println("Error marshalling allcps")
				return nil, err1
			}
			fmt.Println("All success, returning allcps")
			return allCPsBytes, nil
		}
	} else if args[0] == "GetCP" {
		fmt.Println("Getting particular cp")
		cp, err := GetCP(args[1], stub)
		if err != nil {
			fmt.Println("Error Getting particular cp")
			return nil, err
		} else {
			cpBytes, err1 := json.Marshal(&cp)
			if err1 != nil {
				fmt.Println("Error marshalling the cp")
				return nil, err1
			}
			fmt.Println("All success, returning the cp")
			return cpBytes, nil
		}
	} else if args[0] == "GetCompany" {
		fmt.Println("Getting the company")
		company, err := GetCompany(args[1], stub)
		if err != nil {
			fmt.Println("Error from getCompany")
			return nil, err
		} else {
			companyBytes, err1 := json.Marshal(&company)
			if err1 != nil {
				fmt.Println("Error marshalling the company")
				return nil, err1
			}
			fmt.Println("All success, returning the company")
			return companyBytes, nil
		}
	} else {
		fmt.Println("Generic Query call")
		bytes, err := stub.GetState(args[0])

		if err != nil {
			fmt.Println("Some error happenend")
			return nil, errors.New("Some Error happened")
		}

		fmt.Println("All success, returning from generic")
		return bytes, nil
	}
}

func (t *SimpleChaincode) Run(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	fmt.Println("run is running " + function)
	return t.Invoke(stub, function, args)
}

func (t *SimpleChaincode) Invoke(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	fmt.Println("invoke is running " + function)

	if function == "createCoffeeAsset" {
		fmt.Println("Firing createCoffeeAsset")
		//Create an createCoffeeAsset with some value
		return t.createCoffeeAsset(stub, args)
	} else if function == "transferCoffeeAsset" {
		fmt.Println("Firing transferCoffeeAsset")
		return t.transferCoffeeAsset(stub, args)
	} else if function == "issueCommercialPaper" {
		fmt.Println("Firing issueCommercialPaper")
		//Create an asset with some value
		return t.issueCommercialPaper(stub, args)
	} else if function == "transferPaper" {
		fmt.Println("Firing cretransferPaperateAccounts")
		return t.transferPaper(stub, args)
	} else if function == "createAccounts" {
		fmt.Println("Firing createAccounts")
		return t.createAccounts(stub, args)
	} else if function == "createAccount" {
		fmt.Println("Firing createAccount")
		return t.createAccount(stub, args)
	} else if function == "testCreateCoffeeAsset" {
		fmt.Println("Firing testCreateCoffeeAsset")
		return t.testCreateCoffeeAsset(stub, args)
	} else if function == "init" {
		fmt.Println("Firing init")
		return t.Init(stub, "init", args)
	} else if function == "query" {
		fmt.Println("Firing query")
		return t.Query(stub, "query", args)
	}

	return nil, errors.New("Received unknown function invocation")
}

func main() {
	err := shim.Start(new(SimpleChaincode))
	if err != nil {
		fmt.Println("Error starting Simple chaincode: %s", err)
	}
}

//lookup tables for last two digits of CUSIP
var seventhDigit = map[int]string{
	1:  "A",
	2:  "B",
	3:  "C",
	4:  "D",
	5:  "E",
	6:  "F",
	7:  "G",
	8:  "H",
	9:  "J",
	10: "K",
	11: "L",
	12: "M",
	13: "N",
	14: "P",
	15: "Q",
	16: "R",
	17: "S",
	18: "T",
	19: "U",
	20: "V",
	21: "W",
	22: "X",
	23: "Y",
	24: "Z",
}

var eigthDigit = map[int]string{
	1:  "1",
	2:  "2",
	3:  "3",
	4:  "4",
	5:  "5",
	6:  "6",
	7:  "7",
	8:  "8",
	9:  "9",
	10: "A",
	11: "B",
	12: "C",
	13: "D",
	14: "E",
	15: "F",
	16: "G",
	17: "H",
	18: "J",
	19: "K",
	20: "L",
	21: "M",
	22: "N",
	23: "P",
	24: "Q",
	25: "R",
	26: "S",
	27: "T",
	28: "U",
	29: "V",
	30: "W",
	31: "X",
}
