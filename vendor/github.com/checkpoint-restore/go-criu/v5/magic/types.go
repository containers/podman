package magic

const (
	ImgCommonMagic  = 0x54564319 /* Sarov (a.k.a. Arzamas-16) */
	ImgServiceMagic = 0x55105940 /* Zlatoust */
	StatsMagic      = 0x57093306 /* Ostashkov */

	PrimaryMagicOffset   = 0x0
	SecondaryMagicOffset = 0x4
	SizeOffset           = 0x8
	PayloadOffset        = 0xC
)
