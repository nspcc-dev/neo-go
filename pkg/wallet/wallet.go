package wallet

// Wallet is a NEP-2 compatible NEO wallet.
// It holds the public and private keys along with some metadata.
type Wallet struct {
	// NEO private key.
	PrivateKey *PrivateKey

	// NEO public key.
	PublicKey []byte

	// Signature used with the private key.
	Signature []byte

	// NEO public addresss.
	Address string

	// Wallet import file.
	WIF string

	// Encrypted wallet import file.
	EncryptedWIF string
}

// New creates a new Wallet with a random generated PrivateKey.
func New() (*Wallet, error) {
	priv, err := NewPrivateKey()
	if err != nil {
		return nil, err
	}

	return newFromPrivateKey(priv)
}

// Decrypt decrypt the encryptedWIF with the given passphrase and
// return the decrypted Wallet.
func Decrypt(encryptedWIF, passphrase string) (*Wallet, error) {
	wif, err := NEP2Decrypt(encryptedWIF, passphrase)
	if err != nil {
		return nil, err
	}
	return NewFromWIF(wif)
}

// Encrypt encrypts the wallet's PrivateKey with the given passphrase
// under the NEP-2 standard.
func (w *Wallet) Encrypt(passphrase string) error {
	wif, err := NEP2Encrypt(w.PrivateKey, passphrase)
	if err != nil {
		return err
	}
	w.EncryptedWIF = wif
	return nil
}

// NewFromWIF creates a new Wallet from the given WIF.
func NewFromWIF(wif string) (*Wallet, error) {
	privKey, err := NewPrivateKeyFromWIF(wif)
	if err != nil {
		return nil, err
	}
	return newFromPrivateKey(privKey)
}

// newFromPrivateKey created a wallet from the given PrivateKey.
func newFromPrivateKey(p *PrivateKey) (*Wallet, error) {
	pubKey, err := p.PublicKey()
	if err != nil {
		return nil, err
	}
	pubAddr, err := p.Address()
	if err != nil {
		return nil, err
	}
	sig, err := p.Signature()
	if err != nil {
		return nil, err
	}
	wif, err := p.WIF()
	if err != nil {
		return nil, err
	}

	w := &Wallet{
		PublicKey:  pubKey,
		PrivateKey: p,
		Address:    pubAddr,
		Signature:  sig,
		WIF:        wif,
	}

	return w, nil
}
