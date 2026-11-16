package policy

type Hardfork string

// The list of valid hardfork names managed by Policy contract starting from Huyao
// hardfork.
const (
	HFAspidochelone Hardfork = "Aspidochelone"
	HFBasilisk      Hardfork = "Basilisk"
	HFCockatrice    Hardfork = "Cockatrice"
	HFDomovoi       Hardfork = "Domovoi"
	HFEchidna       Hardfork = "Echidna"
	HFFaun          Hardfork = "Faun"
	HFGorgon        Hardfork = "Gorgon"
	HFHuyao         Hardfork = "Huyao"
	HFIara          Hardfork = "Iara"
	HFJackalope     Hardfork = "Jackalope"
)
