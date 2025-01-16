package spscqueue

import (
	"testing"
)

func TestRoundPowerOfTwo(t *testing.T) {
	type args struct {
		n uint64
	}
	tests := []struct {
		name string
		args args
		want uint64
	}{
		{
			args: args{
				n: 1023,
			},
			want: 1024,
		},
		{
			args: args{
				n: 1024,
			},
			want: 1024,
		},
		{
			args: args{
				n: 2048,
			},
			want: 2048,
		},
		{
			args: args{
				n: 2049,
			},
			want: 4096,
		},
		{
			args: args{
				n: 0xffffffff,
			},
			want: 0x100000000,
		},
		{
			args: args{
				n: 4096,
			},
			want: 4096,
		},
		{
			args: args{
				n: 4095,
			},
			want: 4096,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RoundPowerOfTwo(tt.args.n); got != tt.want {
				t.Errorf("RoundPowerOfTwo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsPowerOfTwo(t *testing.T) {
	type args struct {
		n uint64
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			args: args{
				n: 100,
			},
			want: false,
		},
		{
			args: args{
				n: 128,
			},
			want: true,
		},
		{
			args: args{
				n: 0,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPowerOfTwo(tt.args.n); got != tt.want {
				t.Errorf("IsPowerOfTwo() = %v, want %v", got, tt.want)
			}
		})
	}
}
