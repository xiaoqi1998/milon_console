// handler/bulk_transfer_handler_test.go
package handler

import "testing"

// bulkTransferAmount 按余额百分比归集(整数最小单位,向下取整)。
// 2026-11-25 水龙头降为单次 1 MIL,原写死 9800 MIL 归集在新规则下恒 513。
func TestBulkTransferAmount(t *testing.T) {
	cases := []struct {
		name    string
		bal     uint64
		percent uint64
		want    uint64
	}{
		{"1MIL 转 98%", 1_000_000, 98, 980_000},
		{"零余额", 0, 98, 0},
		{"零头向下取整", 12345, 98, 12098},
		{"百分百全转", 1_000_000, 100, 1_000_000},
		{"百分之一", 1_000_000, 1, 10_000},
		{"极小余额", 99, 98, 97},
	}
	for _, tc := range cases {
		if got := bulkTransferAmount(tc.bal, tc.percent); got != tc.want {
			t.Errorf("%s: bulkTransferAmount(%d,%d) = %d, want %d", tc.name, tc.bal, tc.percent, got, tc.want)
		}
	}
}

// normalizeTransferPercent 缺省 98;合法范围 1-100;越界报错。
func TestNormalizeTransferPercent(t *testing.T) {
	cases := []struct {
		name    string
		in      int
		want    uint64
		wantErr bool
	}{
		{"缺省归 98", 0, 98, false},
		{"显式 98", 98, 98, false},
		{"下界 1", 1, 1, false},
		{"上界 100", 100, 100, false},
		{"负数拒绝", -1, 0, true},
		{"超上界拒绝", 101, 0, true},
	}
	for _, tc := range cases {
		got, err := normalizeTransferPercent(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("%s: 应报错, got %d", tc.name, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: 不应报错, got %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
}
