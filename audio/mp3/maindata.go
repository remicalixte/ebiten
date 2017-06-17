// Copyright 2017 The Ebiten Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build !js

package mp3

import (
	"fmt"
	"io"
)

var mpeg1_scalefac_sizes = [16][2]int{
	{0, 0}, {0, 1}, {0, 2}, {0, 3}, {3, 0}, {1, 1}, {1, 2}, {1, 3},
	{2, 1}, {2, 2}, {2, 3}, {3, 1}, {3, 2}, {3, 3}, {4, 2}, {4, 3},
}

func (f *frame) readMainL3() error {
	nch := f.numberOfChannels()
	/* Calculate header audio data size */
	framesize := (144*
		g_mpeg1_bitrates[f.header.layer][f.header.bitrate_index])/
		g_sampling_frequency[f.header.sampling_frequency] +
		int(f.header.padding_bit)

	if framesize > 2000 {
		return fmt.Errorf("mp3: framesize = %d", framesize)
	}
	/* Sideinfo is 17 bytes for one channel and 32 bytes for two */
	sideinfo_size := 32
	if nch == 1 {
		sideinfo_size = 17
	}
	/* Main data size is the rest of the frame,including ancillary data */
	main_data_size := framesize - sideinfo_size - 4 /* sync+header */
	/* CRC is 2 bytes */
	if f.header.protection_bit == 0 {
		main_data_size -= 2
	}
	/* Assemble main data buffer with data from this frame and the previous
	 * two frames. main_data_begin indicates how many bytes from previous
	 * frames that should be used. This buffer is later accessed by the
	 * getMainBits function in the same way as the side info is.
	 */
	if err := getMainData(main_data_size, int(f.sideInfo.main_data_begin)); err != nil {
		/* This could be due to not enough data in reservoir */
		return err
	}
	for gr := 0; gr < 2; gr++ {
		for ch := 0; ch < nch; ch++ {
			part_2_start := getMainPos()
			/* Number of bits in the bitstream for the bands */
			slen1 := mpeg1_scalefac_sizes[f.sideInfo.scalefac_compress[gr][ch]][0]
			slen2 := mpeg1_scalefac_sizes[f.sideInfo.scalefac_compress[gr][ch]][1]
			if (f.sideInfo.win_switch_flag[gr][ch] != 0) && (f.sideInfo.block_type[gr][ch] == 2) {
				if f.sideInfo.mixed_block_flag[gr][ch] != 0 {
					for sfb := 0; sfb < 8; sfb++ {
						f.mainData.scalefac_l[gr][ch][sfb] = getMainBits(slen1)
					}
					for sfb := 3; sfb < 12; sfb++ {
						/*slen1 for band 3-5,slen2 for 6-11*/
						nbits := slen2
						if sfb < 6 {
							nbits = slen1
						}
						for win := 0; win < 3; win++ {
							f.mainData.scalefac_s[gr][ch][sfb][win] = getMainBits(nbits)
						}
					}
				} else {
					for sfb := 0; sfb < 12; sfb++ {
						/*slen1 for band 3-5,slen2 for 6-11*/
						nbits := slen2
						if sfb < 6 {
							nbits = slen1
						}
						for win := 0; win < 3; win++ {
							f.mainData.scalefac_s[gr][ch][sfb][win] = getMainBits(nbits)
						}
					}
				}
			} else { /* block_type == 0 if winswitch == 0 */
				/* Scale factor bands 0-5 */
				if (f.sideInfo.scfsi[ch][0] == 0) || (gr == 0) {
					for sfb := 0; sfb < 6; sfb++ {
						f.mainData.scalefac_l[gr][ch][sfb] = getMainBits(slen1)
					}
				} else if (f.sideInfo.scfsi[ch][0] == 1) && (gr == 1) {
					/* Copy scalefactors from granule 0 to granule 1 */
					for sfb := 0; sfb < 6; sfb++ {
						f.mainData.scalefac_l[1][ch][sfb] = f.mainData.scalefac_l[0][ch][sfb]
					}
				}
				/* Scale factor bands 6-10 */
				if (f.sideInfo.scfsi[ch][1] == 0) || (gr == 0) {
					for sfb := 6; sfb < 11; sfb++ {
						f.mainData.scalefac_l[gr][ch][sfb] = getMainBits(slen1)
					}
				} else if (f.sideInfo.scfsi[ch][1] == 1) && (gr == 1) {
					/* Copy scalefactors from granule 0 to granule 1 */
					for sfb := 6; sfb < 11; sfb++ {
						f.mainData.scalefac_l[1][ch][sfb] = f.mainData.scalefac_l[0][ch][sfb]
					}
				}
				/* Scale factor bands 11-15 */
				if (f.sideInfo.scfsi[ch][2] == 0) || (gr == 0) {
					for sfb := 11; sfb < 16; sfb++ {
						f.mainData.scalefac_l[gr][ch][sfb] = getMainBits(slen2)
					}
				} else if (f.sideInfo.scfsi[ch][2] == 1) && (gr == 1) {
					/* Copy scalefactors from granule 0 to granule 1 */
					for sfb := 11; sfb < 16; sfb++ {
						f.mainData.scalefac_l[1][ch][sfb] = f.mainData.scalefac_l[0][ch][sfb]
					}
				}
				/* Scale factor bands 16-20 */
				if (f.sideInfo.scfsi[ch][3] == 0) || (gr == 0) {
					for sfb := 16; sfb < 21; sfb++ {
						f.mainData.scalefac_l[gr][ch][sfb] = getMainBits(slen2)
					}
				} else if (f.sideInfo.scfsi[ch][3] == 1) && (gr == 1) {
					/* Copy scalefactors from granule 0 to granule 1 */
					for sfb := 16; sfb < 21; sfb++ {
						f.mainData.scalefac_l[1][ch][sfb] = f.mainData.scalefac_l[0][ch][sfb]
					}
				}
			}
			/* Read Huffman coded data. Skip stuffing bits. */
			if err := f.readHuffman(part_2_start, gr, ch); err != nil {
				return err
			}
		}
	}
	/* The ancillary data is stored here,but we ignore it. */
	return nil
}

type mainDataBytes struct {
	// Large static data
	vec [2 * 1024]int
	// Pointer into the reservoir
	ptr []int
	// Index into the current byte(0-7)
	idx int
	// Number of bytes in reservoir(0-1024)
	top int

	pos int
}

var theMainDataBytes mainDataBytes

func getMainData(size int, begin int) error {
	if size > 1500 {
		return fmt.Errorf("mp3: size = %d", size)
	}
	// Check that there's data available from previous frames if needed
	if int(begin) > theMainDataBytes.top {
		// No,there is not, so we skip decoding this frame, but we have to
		// read the main_data bits from the bitstream in case they are needed
		// for decoding the next frame.
		buf := make([]int, size)
		n := 0
		var err error
		for n < size && err == nil {
			nn, err2 := getBytes(buf)
			n += nn
			err = err2
		}
		if n < size {
			if err == io.EOF {
				return fmt.Errorf("mp3: unexpected EOF at getMainData")
			}
			return err
		}
		copy(theMainDataBytes.vec[theMainDataBytes.top:], buf[:n])
		/* Set up pointers */
		theMainDataBytes.ptr = theMainDataBytes.vec[0:]
		theMainDataBytes.pos = 0
		theMainDataBytes.idx = 0
		theMainDataBytes.top += size
		// TODO: Define a special error and enable to continue the next frame.
		return fmt.Errorf("mp3: frame can't be decoded")
	}
	/* Copy data from previous frames */
	for i := 0; i < begin; i++ {
		theMainDataBytes.vec[i] = theMainDataBytes.vec[theMainDataBytes.top-begin+i]
	}
	/* Read the main_data from file */
	buf := make([]int, size)
	n := 0
	var err error
	for n < size && err == nil {
		nn, err2 := getBytes(buf)
		n += nn
		err = err2
	}
	if n < size {
		if err == io.EOF {
			return fmt.Errorf("mp3: unexpected EOF at getMainData")
		}
		return err
	}
	copy(theMainDataBytes.vec[begin:], buf[:n])
	/* Set up pointers */
	theMainDataBytes.ptr = theMainDataBytes.vec[0:]
	theMainDataBytes.pos = 0
	theMainDataBytes.idx = 0
	theMainDataBytes.top = begin + size
	return nil
}

func getMainBit() int {
	tmp := uint(theMainDataBytes.ptr[0]) >> (7 - uint(theMainDataBytes.idx))
	tmp &= 0x01
	theMainDataBytes.ptr = theMainDataBytes.ptr[(theMainDataBytes.idx+1)>>3:]
	theMainDataBytes.pos += (theMainDataBytes.idx + 1) >> 3
	theMainDataBytes.idx = (theMainDataBytes.idx + 1) & 0x07
	return int(tmp)
}

func getMainBits(num int) int {
	if num == 0 {
		return 0
	}
	/* Form a word of the next four bytes */
	b := make([]int, 4)
	for i := range b {
		if len(theMainDataBytes.ptr) > i {
			b[i] = theMainDataBytes.ptr[i]
		}
	}
	tmp := (uint32(b[0]) << 24) | (uint32(b[1]) << 16) | (uint32(b[2]) << 8) | (uint32(b[3]) << 0)

	/* Remove bits already used */
	tmp = tmp << uint(theMainDataBytes.idx)

	/* Remove bits after the desired bits */
	tmp = tmp >> (32 - uint(num))

	/* Update pointers */
	theMainDataBytes.ptr = theMainDataBytes.ptr[(theMainDataBytes.idx+int(num))>>3:]
	theMainDataBytes.pos += (theMainDataBytes.idx + num) >> 3
	theMainDataBytes.idx = (theMainDataBytes.idx + num) & 0x07

	/* Done */
	return int(tmp)
}

func getMainPos() int {
	pos := theMainDataBytes.pos
	pos *= 8                    /* Multiply by 8 to get number of bits */
	pos += theMainDataBytes.idx /* Add current bit index */
	return pos
}

func setMainPos(bit_pos int) {
	theMainDataBytes.ptr = theMainDataBytes.vec[bit_pos>>3:]
	theMainDataBytes.pos = int(bit_pos) >> 3
	theMainDataBytes.idx = int(bit_pos) & 0x7
}