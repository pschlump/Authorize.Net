package AuthorizeNet

import (
	"log"
	"testing"
)

/*no*/
func AuthExample(t *testing.T) {
	auth := AuthorizeNet{
		Login:     "<YourLogin>", // xyzzy - from config file!
		Key:       "<YourKey>",   // xyzzy - from call to NewAuthrizeNet ( host, Login, Key ) -- with option to read from JSON file
		DupWindow: 120,
		TestMode:  true, // From Config File -- xyzzy
	}

	card := CardInfoType{ // xyzzy - NewCreditCardInfo -- xyzzy
		CreditCardNumber: "4111111111111111", //
		CVV:              "555",
		Month_Year:       "11/2018",
		Method:           METHOD_VISA,
	}

	data := AuthorizeNetType{ // xyzzy NewCreditCardTransaction
		InvoiceNumber: "123",
		Amount:        "5.56",
		Description:   "My Test transaction",
		// ....
		// Fill in the rest of AuthorizeNetType: adress etc ...
		// ....
	}

	// Authorize a payment
	response := auth.Authorize(card, data, false)
	if response.IsApproved {
		log.Print(response)
		return
	}
	log.Print(response)
	log.Printf("Successful Authorization with id: %s ", response.TransId)

	// Example of capture the preious authorization (using the transactionId)
	response = auth.CapturePreauth(response.TransId, "5.56")
	if response.IsApproved {
		log.Print("Capture failed : ")
		log.Print(response)
		return
	}
	log.Print(response)
	log.Print("Successful Capture !")

}
