// High-Level API

package libmyna

import (
	"crypto"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/mozilla-services/pkcs7"
)

var Debug bool

func CheckCard() error {
	reader, err := NewReader()
	if err != nil {
		return err
	}
	defer reader.Finalize()
	reader.SetDebug(Debug)
	err = reader.Connect()
	if err != nil {
		return err
	}

	jpkiAP, err := reader.SelectJPKIAP()
	if err != nil {
		return errors.New("個人番号カードではありません")
	}

	err = reader.SelectEF("00 06")
	if err != nil {
		return errors.New("トークン情報を取得できません")
	}

	token, err := jpkiAP.GetToken()
	if token == "JPKIAPICCTOKEN2" {
		return nil
	} else if token == "JPKIAPICCTOKEN" {
		return errors.New("これは住基カードですね?")
	} else {
		return fmt.Errorf("不明なトークン情報: %s", token)
	}
}

// 券面入力補助APのマイナンバーを取得します
func GetMyNumber(pin string) (string, error) {
	reader, err := NewReader()
	if err != nil {
		return "", err
	}
	defer reader.Finalize()
	reader.SetDebug(Debug)
	err = reader.Connect()
	if err != nil {
		return "", err
	}
	helperAP, err := reader.SelectCardInputHelperAP()
	if err != nil {
		return "", err
	}
	helperAP.VerifyPin(pin)
	if err != nil {
		return "", err
	}

	mynumber, err := helperAP.ReadMyNumber()
	if err != nil {
		return "", err
	}
	return mynumber, nil
}

// 券面入力補助APの4属性情報を取得します
func GetAttrInfo(pin string) (*CardInputHelperAttrs, error) {
	reader, err := NewReader()
	if err != nil {
		return nil, err
	}
	defer reader.Finalize()
	reader.SetDebug(Debug)
	err = reader.Connect()
	if err != nil {
		return nil, err
	}

	helperAP, err := reader.SelectCardInputHelperAP()
	if err != nil {
		return nil, err
	}
	helperAP.VerifyPin(pin)
	if err != nil {
		return nil, err
	}

	attr, err := helperAP.ReadAttrInfo()
	return attr, nil
}

type CardInfo struct {
}

// 券面事項確認AP
func GetCardInfo(pin string) (*CardInfo, error) {
	reader, err := NewReader()
	if err != nil {
		return nil, err
	}
	defer reader.Finalize()
	reader.SetDebug(Debug)
	err = reader.Connect()
	if err != nil {
		return nil, err
	}

	helperAP, err := reader.SelectCardInputHelperAP()
	if err != nil {
		return nil, err
	}
	helperAP.VerifyPin(pin)
	if err != nil {
		return nil, err
	}

	mynumber, err := helperAP.ReadMyNumber()
	if err != nil {
		return nil, err
	}

	cardAP, err := reader.SelectCardInfoAP()
	if err != nil {
		return nil, err
	}
	cardAP.VerifyPinA(mynumber)
	if err != nil {
		return nil, err
	}

	cardAP.Test()

	return nil, nil
}

func ChangeCardInputHelperPin(pin string, newpin string) error {
	return Change4DigitPin(pin, newpin, "CARD_INPUT_HELPER")
}

func ChangeJPKIAuthPin(pin string, newpin string) error {
	return Change4DigitPin(pin, newpin, "JPKI_AUTH")
}

func Change4DigitPin(pin string, newpin string, pintype string) error {

	err := Validate4DigitPin(pin)
	if err != nil {
		return err
	}

	err = Validate4DigitPin(newpin)
	if err != nil {
		return err
	}

	reader, err := NewReader()
	if err != nil {
		return err
	}
	defer reader.Finalize()
	reader.SetDebug(Debug)
	err = reader.Connect()
	if err != nil {
		return err
	}

	switch pintype {
	case "CARD_INPUT_HELPER":
		reader.SelectCardInputHelperAP()
		reader.SelectEF("00 11") // 券面入力補助PIN
	case "JPKI_AUTH":
		reader.SelectJPKIAP()
		reader.SelectEF("00 18") //JPKI認証用PIN
	}

	err = reader.Verify(pin)
	if err != nil {
		return err
	}

	res := reader.ChangePin(newpin)
	if !res {
		return errors.New("PINの変更に失敗しました")
	}
	return nil
}

func ChangeJPKISignPin(pin string, newpin string) error {
	pin = strings.ToUpper(pin)
	err := ValidateJPKISignPassword(pin)
	if err != nil {
		return err
	}

	newpin = strings.ToUpper(newpin)
	err = ValidateJPKISignPassword(newpin)
	if err != nil {
		return err
	}

	reader, err := NewReader()
	if err != nil {
		return err
	}
	defer reader.Finalize()
	reader.SetDebug(Debug)
	err = reader.Connect()
	if err != nil {
		return err
	}

	reader.SelectJPKIAP()
	reader.SelectEF("00 1B") // IEF for SIGN

	err = reader.Verify(pin)
	if err != nil {
		return err
	}

	res := reader.ChangePin(newpin)
	if !res {
		return errors.New("PINの変更に失敗しました")
	}
	return nil
}

func GetJPKICert(efid string, pin string) (*x509.Certificate, error) {
	reader, err := NewReader()
	if err != nil {
		return nil, err
	}
	defer reader.Finalize()
	reader.SetDebug(Debug)
	err = reader.Connect()
	if err != nil {
		return nil, err
	}

	jpkiAP, err := reader.SelectJPKIAP()
	if err != nil {
		return nil, err
	}

	if pin != "" {
		err = jpkiAP.VerifySignPin(pin)
		if err != nil {
			return nil, err
		}
	}
	cert, err := jpkiAP.ReadCertificate(efid)
	return cert, nil
}

func GetJPKIAuthCert() (*x509.Certificate, error) {
	return GetJPKICert("00 0A", "")
}

func GetJPKIAuthCACert() (*x509.Certificate, error) {
	return GetJPKICert("00 0B", "")
}

func GetJPKISignCert(pass string) (*x509.Certificate, error) {
	return GetJPKICert("00 01", pass)
}

func GetJPKISignCACert() (*x509.Certificate, error) {
	return GetJPKICert("00 02", "")
}

/*
func CmsSignJPKISignOld(pin string, in string, out string) error {
	rawContent, err := ioutil.ReadFile(in)
	if err != nil {
		return err
	}

	toBeSigned, err := pkcs7.NewSignedData(rawContent)
	if err != nil {
		return err
	}

	// 署名用証明書の取得
	cert, err := GetJPKISignCert(pin)
	if err != nil {
		return err
	}
	attrs, hashed, err := toBeSigned.HashAttributes(crypto.SHA1, pkcs7.SignerInfoConfig{})
	if err != nil {
		return err
	}

	ias, err := pkcs7.Cert2issuerAndSerial(cert)
	if err != nil {
		return err
	}

	reader, err := NewReader()
	if err != nil {
		return err
	}
	defer reader.Finalize()
	reader.SetDebug(Debug)
	err = reader.Connect()
	if err != nil {
		return err
	}

	reader.SelectJPKIAP()
	reader.SelectEF("00 1B") // IEF for SIGN
	err = reader.Verify(pin)
	if err != nil {
		return err
	}

	reader.SelectEF("00 1A") // Select SIGN EF
	digestInfo := makeDigestInfo(hashed)

	signature, err := reader.Signature(digestInfo)
	if err != nil {
		return err
	}

	oidDigestAlgorithmSHA1 := asn1.ObjectIdentifier{1, 3, 14, 3, 2, 26}
	oidEncryptionAlgorithmRSA := asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}
	signerInfo := pkcs7.SignerInfo{
		AuthenticatedAttributes:   attrs,
		DigestAlgorithm:           pkix.AlgorithmIdentifier{Algorithm: oidDigestAlgorithmSHA1},
		DigestEncryptionAlgorithm: pkix.AlgorithmIdentifier{Algorithm: oidEncryptionAlgorithmRSA},
		IssuerAndSerialNumber:     ias,
		EncryptedDigest:           signature,
		Version:                   1,
	}
	toBeSigned.AddSignerInfo(cert, signerInfo)
	signed, err := toBeSigned.Finish()
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(out, signed, 0664)
	if err != nil {
		return err
	}
	return nil
}
*/

type JPKISignSigner struct {
	pin    string
	pubkey crypto.PublicKey
}

func (self JPKISignSigner) Public() crypto.PublicKey {
	return self.pubkey
}

func (self JPKISignSigner) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	digestInfo := makeDigestInfo(opts.HashFunc(), digest)
	reader, err := NewReader()
	if err != nil {
		return nil, err
	}
	defer reader.Finalize()
	reader.SetDebug(Debug)
	err = reader.Connect()
	if err != nil {
		return nil, err
	}
	reader.SelectJPKIAP()
	reader.SelectEF("00 1B") // IEF for SIGN
	err = reader.Verify(self.pin)
	if err != nil {
		return nil, err
	}

	reader.SelectEF("00 1A") // Select SIGN EF
	signature, err = reader.Signature(digestInfo)
	if err != nil {
		return nil, err
	}
	return signature, nil
}

func GetDigestOID(md string) (asn1.ObjectIdentifier, error) {
	switch strings.ToUpper(md) {
	case "SHA1":
		return pkcs7.OIDDigestAlgorithmSHA1, nil
	case "SHA256":
		return pkcs7.OIDDigestAlgorithmSHA256, nil
	case "SHA384":
		return pkcs7.OIDDigestAlgorithmSHA384, nil
	case "SHA512":
		return pkcs7.OIDDigestAlgorithmSHA512, nil
	default:
		return nil, fmt.Errorf("サポートされていないハッシュアルゴリズムです: %s", md)
	}
}

type CmsSignOpts struct {
	Hash string
	Form string
}

func CmsSignJPKISign(pin string, in string, out string, opts CmsSignOpts) error {
	digest, err := GetDigestOID(opts.Hash)
	if err != nil {
		return err
	}

	content, err := ioutil.ReadFile(in)
	if err != nil {
		return err
	}

	// 署名用証明書の取得
	cert, err := GetJPKISignCert(pin)
	if err != nil {
		return err
	}

	privkey := JPKISignSigner{pin, cert.PublicKey}

	toBeSigned, err := pkcs7.NewSignedData(content)
	toBeSigned.SetDigestAlgorithm(digest)
	err = toBeSigned.AddSigner(cert, privkey, pkcs7.SignerInfoConfig{})
	if err != nil {
		return err
	}

	signed, err := toBeSigned.Finish()
	if err != nil {
		return err
	}

	if err = writeCms(out, signed, opts.Form); err != nil {
		return err
	}

	return nil
}

func writeCms(out string, signed []byte, form string) error {
	var file *os.File
	var err error
	if out == "" {
		file = os.Stdout
	} else {
		file, err = os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		defer file.Close()
		if err != nil {
			return err
		}
	}

	switch strings.ToUpper(form) {
	case "PEM":
		err = pem.Encode(file, &pem.Block{Type: "PKCS7", Bytes: signed})
		if err != nil {
			return err
		}

	case "DER":
		_, err = file.Write(signed)
		if err != nil {
			return err
		}
	}
	return nil
}

func readCMSFile(in string, form string) (*pkcs7.PKCS7, error) {
	data, err := ioutil.ReadFile(in)
	if err != nil {
		return nil, err
	}

	var signedDer []byte
	switch strings.ToUpper(form) {
	case "PEM":
		block, _ := pem.Decode(data)
		signedDer = block.Bytes
	case "DER":
		signedDer = data
	default:
		return nil, fmt.Errorf("サポートされていない形式です: %s", form)
	}

	p7, err := pkcs7.Parse(signedDer)
	if err != nil {
		return nil, err
	}
	return p7, nil
}

func CmsVerifyJPKISign(in string, form string) error {
	cacert, err := GetJPKISignCACert()
	if err != nil {
		return err
	}
	p7, err := readCMSFile(in, form)
	if err != nil {
		return err
	}

	certPool := x509.NewCertPool()
	certPool.AddCert(cacert)
	err = p7.VerifyWithChain(certPool)
	if err != nil {
		return err
	}

	return nil
}
