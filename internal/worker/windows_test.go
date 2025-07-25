package worker_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/worker"
)

// Windows names taken from https://dp-downloads.broadcom.com/api-content/apis/API_VWSA_001/8.0U3/html/ReferenceGuides/vim.vm.GuestOsDescriptor.GuestOsIdentifier.html
// Distrobuilder Windows identifiers taken from https://github.com/lxc/distrobuilder/blob/4937e157abcdbc55ad9f2c5a58bb827356d4ec8c/windows/wiminfo.go#L13
func TestMapWindowsVersionToAbbrevSuccess(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{
			name: "Windows 11",
			want: "w11",
		},
		{
			name: "Windows Server 2019",
			want: "2k19",
		},
		{
			name: "Windows Server 2022",
			want: "2k22",
		},
		{
			name: "Windows Server 2025",
			want: "2k25",
		},
		{
			name: "Windows 7 (64 bit)",
			want: "w7",
		},
		{
			name: "Windows 7",
			want: "w7",
		},
		{
			name: "Windows Server 2008 R2 (64 bit)",
			want: "2k8r2",
		},
		{
			name: "Windows 8 (64 bit)",
			want: "w8",
		},
		{
			name: "Windows 8",
			want: "w8",
		},
		{
			name: "Windows 8 Server (64 bit)",
			want: "w8",
		},
		{
			name: "Windows 10 (64 bit)",
			want: "w10",
		},
		{
			name: "Windows 10",
			want: "w10",
		},
		{
			name: "Windows 10 Server (64 bit)",
			want: "w10",
		},
		{
			name: "Windows Small Business Server 2003",
			want: "2k3",
		},
		{
			name: "Windows Server 2003, Datacenter Edition (64 bit)",
			want: "2k3",
		},
		{
			name: "Windows Server 2003, Datacenter Edition",
			want: "2k3",
		},
		{
			name: "Windows Server 2003, Enterprise Edition (64 bit)",
			want: "2k3",
		},
		{
			name: "Windows Server 2003, Enterprise Edition",
			want: "2k3",
		},
		{
			name: "Windows Server 2003, Standard Edition (64 bit)",
			want: "2k3",
		},
		{
			name: "Windows Server 2003, Standard Edition",
			want: "2k3",
		},
		{
			name: "Windows Server 2003, Web Edition",
			want: "2k3",
		},
		{
			name: "Windows XP Home Edition",
			want: "xp",
		},
		{
			name: "Windows XP Professional Edition (64 bit)",
			want: "xp",
		},
		{
			name: "Windows XP Professional",
			want: "xp",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := worker.MapWindowsVersionToAbbrev(tc.name)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// Windows names taken from https://dp-downloads.broadcom.com/api-content/apis/API_VWSA_001/8.0U3/html/ReferenceGuides/vim.vm.GuestOsDescriptor.GuestOsIdentifier.html
func TestMapWindowsVersionToAbbrevNotSupported(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "Windows 2000 Advanced Server",
		},
		{
			name: "Windows 2000 Professional",
		},
		{
			name: "Windows 2000 Server",
		},
		{
			name: "Windows 3.1",
		},
		{
			name: "Windows 95",
		},
		{
			name: "Windows 98",
		},
		{
			name: "Windows 12",
		},
		{
			name: "Windows Hyper-V",
		},
		{
			name: "Windows Longhorn (64 bit)",
		},
		{
			name: "Windows Longhorn",
		},
		{
			name: "Windows Millennium Edition",
		},
		{
			name: "Windows NT 4",
		},
		{
			name: "Windows Vista (64 bit)",
		},
		{
			name: "Windows Vista",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := worker.MapWindowsVersionToAbbrev(tc.name)
			require.Error(t, err)
		})
	}
}
