package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/Eyevinn/mp4ff/avc"
	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/samplebuilder"
)

func main() {
	// listen on port 5004
	conn, err := net.ListenPacket("udp", ":5004")
	if err != nil {
		panic(err)
	}

	builder := samplebuilder.New(10, &codecs.H264Packet{}, 90000)

	fmp4 := &FMP4Builder{}

	for {
		buf := make([]byte, 1500) // max rtp packet size

		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}

		p := &rtp.Packet{}

		if err := p.Unmarshal(buf[:n]); err != nil {
			panic(err)
		}

		builder.Push(p)

		for {
			ms := builder.Pop()
			if ms == nil {
				break
			}
			if err := fmp4.Push(ms); err != nil {
				panic(err)
			}
		}
	}
}

type FMP4Builder struct {
	sps, pps [][]byte
	mss      []*media.Sample

	i int
}

func (b *FMP4Builder) Flush() error {
	if b.sps == nil || b.pps == nil {
		return nil
	}

	f, err := os.Create(fmt.Sprintf("output-%05d.mp4", b.i))
	if err != nil {
		return err
	}

	init := mp4.CreateEmptyInit()
	init.AddEmptyTrack(12800, "video", "en")
	if err := init.Moov.Trak.SetAVCDescriptor("avc1", b.sps, b.pps, true); err != nil {
		return err
	}
	if err := init.Encode(f); err != nil {
		return err
	}

	seg := mp4.NewMediaSegment()
	frag, err := mp4.CreateFragment(1, init.Moov.Trak.Tkhd.TrackID)
	if err != nil {
		return err
	}
	seg.AddFragment(frag)

	for _, ms := range b.mss {
		frag.AddFullSample(mp4.FullSample{
			Sample: mp4.Sample{
				Flags:                 0,
				Dur:                   uint32(ms.Duration.Seconds() * 12800),
				Size:                  uint32(len(ms.Data)),
				CompositionTimeOffset: 0,
			},
			DecodeTime: uint64(ms.Timestamp.UnixMicro() * 12800 / 1000000),
			Data:       ms.Data,
		})
	}

	if err := seg.Encode(f); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	b.i++

	b.sps = nil
	b.pps = nil
	b.mss = nil

	return nil
}

func (b *FMP4Builder) Push(ms *media.Sample) error {
	sample := avc.ConvertByteStreamToNaluSample(ms.Data)

	if avc.HasParameterSets(sample) {
		if err := b.Flush(); err != nil {
			return err
		}
		sampleLength := uint32(len(sample))
		extraNalu := []byte{}
		var pos uint32 = 0
		for pos < sampleLength {
			naluLength := binary.BigEndian.Uint32(sample[pos : pos+4])
			pos += 4
			naluHdr := sample[pos]
			switch naluType := avc.GetNaluType(naluHdr); {
			case naluType == avc.NALU_SPS:
				b.sps = append(b.sps, sample[pos:pos+naluLength])
			case naluType == avc.NALU_PPS:
				b.pps = append(b.pps, sample[pos:pos+naluLength])
			default:
				extraNalu = append(extraNalu, sample[pos-4:pos+naluLength]...)
			}
			pos += naluLength
		}
		ms.Data = extraNalu
	}
	b.mss = append(b.mss, ms)

	return nil
}
