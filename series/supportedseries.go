// Copyright 2014 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package series

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/juju/errors"
	"github.com/juju/loggo"
	"github.com/juju/os/v2"
)

var (
	// TODO(katco): Remove globals (lp:1633571)
	logger = loggo.GetLogger("juju.juju.series")
)

type unknownOSForSeriesError string

func (e unknownOSForSeriesError) Error() string {
	return `unknown OS for series: "` + string(e) + `"`
}

// IsUnknownOSForSeriesError returns true if err is of type unknownOSForSeriesError.
func IsUnknownOSForSeriesError(err error) bool {
	_, ok := errors.Cause(err).(unknownOSForSeriesError)
	return ok
}

type unknownSeriesVersionError string

func (e unknownSeriesVersionError) Error() string {
	return `unknown version for series: "` + string(e) + `"`
}

// IsUnknownSeriesVersionError returns true if err is of type unknownSeriesVersionError.
func IsUnknownSeriesVersionError(err error) bool {
	_, ok := errors.Cause(err).(unknownSeriesVersionError)
	return ok
}

type unknownVersionSeriesError string

func (e unknownVersionSeriesError) Error() string {
	return `unknown series for version: "` + string(e) + `"`
}

// IsUnknownVersionSeriesError returns true if err is of type unknownVersionSeriesError.
func IsUnknownVersionSeriesError(err error) bool {
	_, ok := errors.Cause(err).(unknownVersionSeriesError)
	return ok
}

// seriesVersions provides a mapping between series names and versions.
// The values here are current as of the time of writing. On Ubuntu systems, we update
// these values from /usr/share/distro-info/ubuntu.csv to ensure we have the latest values.
// On non-Ubuntu systems, these values provide a nice fallback option.
// Exported so tests can change the values to ensure the distro-info lookup works.
var seriesVersions = map[string]string{
	"precise":          "12.04",
	"quantal":          "12.10",
	"raring":           "13.04",
	"saucy":            "13.10",
	"trusty":           "14.04",
	"utopic":           "14.10",
	"vivid":            "15.04",
	"wily":             "15.10",
	"xenial":           "16.04",
	"yakkety":          "16.10",
	"zesty":            "17.04",
	"artful":           "17.10",
	"bionic":           "18.04",
	"cosmic":           "18.10",
	"disco":            "19.04",
	"eoan":             "19.10",
	"focal":            "20.04",
	"groovy":           "20.10",
	"hirsute":          "21.04",
	"impish":           "21.10",
	"jammy":            "22.04",
	"kinetic":          "22.10",
	"lunar":            "23.04",
	"mantic":           "23.10",
	"win2008r2":        "win2008r2",
	"win2012hvr2":      "win2012hvr2",
	"win2012hv":        "win2012hv",
	"win2012r2":        "win2012r2",
	"win2012":          "win2012",
	"win2016":          "win2016",
	"win2016hv":        "win2016hv",
	"win2016nano":      "win2016nano",
	"win2019":          "win2019",
	"win7":             "win7",
	"win8":             "win8",
	"win81":            "win81",
	"win10":            "win10",
	"centos7":          "centos7",
	"centos8":          "centos8",
	"centos9":          "centos9",
	"opensuseleap":     "opensuse42",
	genericLinuxSeries: genericLinuxVersion,
}

// versionSeries provides a mapping between versions and series names.
var versionSeries = reverseSeriesVersion()

var centosSeries = map[string]string{
	"centos7": "centos7",
	"centos8": "centos8",
	"centos9": "centos9",
}

var opensuseSeries = map[string]string{
	"opensuseleap": "opensuse42",
}

var kubernetesSeries = map[string]string{
	"kubernetes": "kubernetes",
}

// SeriesVersionInfo represents a ubuntu series that includes the version, if the
// series is an LTS and the supported defines if Juju supports the series
// version.
type SeriesVersionInfo struct {
	Version string
	// LTS provides a lookup for a LTS series.  Like seriesVersions,
	// the values here are current at the time of writing.
	LTS bool
	// Supported defines if Juju classifies the series as officially supported.
	Supported bool
	// Extended security maintenance for customers, extends the supported bool
	// for how Juju classifies the series.
	ESMSupported bool
	// WarningInfo shows any potential issues when parsing the series version
	// information.
	WarningInfo []string
	// CreatedByLocalDistroInfo indicates that the series version was created
	// by the local distro-info information on the system.
	// This is useful to understand why a version appears yet is not supported.
	CreatedByLocalDistroInfo bool
}

var ubuntuSeries = map[string]SeriesVersionInfo{
	"precise": {
		Version: "12.04",
	},
	"quantal": {
		Version: "12.10",
	},
	"raring": {
		Version: "13.04",
	},
	"saucy": {
		Version: "13.10",
	},
	"trusty": {
		Version:      "14.04",
		LTS:          true,
		ESMSupported: true,
	},
	"utopic": {
		Version: "14.10",
	},
	"vivid": {
		Version: "15.04",
	},
	"wily": {
		Version: "15.10",
	},
	"xenial": {
		Version:      "16.04",
		LTS:          true,
		Supported:    true,
		ESMSupported: true,
	},
	"yakkety": {
		Version: "16.10",
	},
	"zesty": {
		Version: "17.04",
	},
	"artful": {
		Version: "17.10",
	},
	"bionic": {
		Version:      "18.04",
		LTS:          true,
		Supported:    true,
		ESMSupported: true,
	},
	"cosmic": {
		Version: "18.10",
	},
	"disco": {
		Version: "19.04",
	},
	"eoan": {
		Version: "19.10",
	},
	"focal": {
		Version:      "20.04",
		LTS:          true,
		Supported:    true,
		ESMSupported: true,
	},
	"groovy": {
		Version:   "20.10",
		Supported: true,
	},
	"hirsute": {
		Version:   "21.04",
		Supported: false,
	},
	"impish": {
		Version:   "21.10",
		Supported: false,
	},
	"jammy": {
		Version:      "22.04",
		LTS:          true,
		Supported:    false,
		ESMSupported: false,
	},
	"kinetic": {
		Version:   "22.10",
		Supported: false,
	},
	"lunar": {
		Version:   "23.04",
		Supported: true,
	},
	"mantic": {
		Version:   "23.10",
		Supported: true,
	},
}

var nonUbuntuSeries = map[string]SeriesVersionInfo{
	"win2008r2": {
		Version:   "win2008r2",
		Supported: true,
	},
	"win2012hvr2": {
		Version:   "win2012hvr2",
		Supported: true,
	},
	"win2012hv": {
		Version:   "win2012hv",
		Supported: true,
	},
	"win2012r2": {
		Version:   "win2012r2",
		Supported: true,
	},
	"win2012": {
		Version:   "win2012",
		Supported: true,
	},
	"win2016": {
		Version:   "win2016",
		Supported: true,
	},
	"win2016hv": {
		Version:   "win2016hv",
		Supported: true,
	},
	"win2016nano": {
		Version:   "win2016nano",
		Supported: true,
	},
	"win2019": {
		Version:   "win2019",
		Supported: true,
	},
	"win7": {
		Version:   "win7",
		Supported: true,
	},
	"win8": {
		Version:   "win8",
		Supported: true,
	},
	"win81": {
		Version:   "win81",
		Supported: true,
	},
	"win10": {
		Version:   "win10",
		Supported: true,
	},
	"centos7": {
		Version:   "centos7",
		Supported: true,
	},
	"centos8": {
		Version:   "centos8",
		Supported: true,
	},
	"centos9": {
		Version:   "centos9",
		Supported: true,
	},
	"opensuseleap": {
		Version:   "opensuse42",
		Supported: true,
	},
	genericLinuxSeries: {
		Version:   genericLinuxVersion,
		Supported: true,
	},
	"kubernetes": {
		Version:   "kubernetes",
		Supported: true,
	},
}

// Windows versions come in various flavors:
// Standard, Datacenter, etc. We use string prefix match them to one
// of the following. Specify the longest name in a particular series first
// For example, if we have "Win 2012" and "Win 2012 R2", we specify "Win 2012 R2" first.
// We need to make sure we manually update this list with each new windows release.
var windowsVersionMatchOrder = []string{
	"Hyper-V Server 2012 R2",
	"Hyper-V Server 2012",
	"Windows Server 2008 R2",
	"Windows Server 2012 R2",
	"Windows Server 2012",
	"Hyper-V Server 2016",
	"Windows Server 2016",
	"Windows Server 2019",
	"Windows Storage Server 2012 R2",
	"Windows Storage Server 2012",
	"Windows Storage Server 2016",
	"Windows 7",
	"Windows 8.1",
	"Windows 8",
	"Windows 10",
}

// windowsVersions is a mapping consisting of the output from
// the following WMI query: (gwmi Win32_OperatingSystem).Name
var windowsVersions = map[string]string{
	"Hyper-V Server 2012 R2":         "win2012hvr2",
	"Hyper-V Server 2012":            "win2012hv",
	"Windows Server 2008 R2":         "win2008r2",
	"Windows Server 2012 R2":         "win2012r2",
	"Windows Server 2012":            "win2012",
	"Hyper-V Server 2016":            "win2016hv",
	"Windows Server 2016":            "win2016",
	"Windows Server 2019":            "win2019",
	"Windows Storage Server 2012 R2": "win2012r2",
	"Windows Storage Server 2012":    "win2012",
	"Windows Storage Server 2016":    "win2016",
	"Windows Storage Server 2019":    "win2019",
	"Windows 7":                      "win7",
	"Windows 8.1":                    "win81",
	"Windows 8":                      "win8",
	"Windows 10":                     "win10",
}

// windowsNanoVersions is a mapping from the product name
// stored in registry to a juju defined nano-series
// On the nano version so far the product name actually
// is identical to the correspondent main windows version
// and the information about it being nano is stored in
// a different place.
var windowsNanoVersions = map[string]string{
	"Windows Server 2016": "win2016nano",
}

// WindowsVersions returns all windows versions as a map
// If we have nan and windows version in common, nano takes precedence
func WindowsVersions() map[string]string {
	save := make(map[string]string)
	for i, val := range windowsVersions {
		save[i] = val
	}

	for i, val := range windowsNanoVersions {
		save[i] = val
	}
	return save
}

func OverwrittenWindowsVersions() []string {
	var overwrittenValues []string
	for i, _ := range windowsNanoVersions {
		if overwritten, ok := windowsVersions[i]; ok {
			overwrittenValues = append(overwrittenValues, overwritten)
		}
	}
	return overwrittenValues
}

// IsWindowsNano tells us whether the provided series is a
// nano series. It may seem futile at this point, but more
// nano series will come up with time.
// This is here and not in a windows specific package
// because we might want to take decisions dependant on
// whether we have a nano series or not in more general code.
func IsWindowsNano(series string) bool {
	for _, val := range windowsNanoVersions {
		if val == series {
			return true
		}
	}
	return false
}

// GetOSFromSeries will return the operating system based
// on the series that is passed to it
func GetOSFromSeries(series string) (os.OSType, error) {
	if series == "" {
		return os.Unknown, errors.NotValidf("series %q", series)
	}
	osType, err := getOSFromSeries(series)
	if err == nil {
		return osType, nil
	}

	seriesVersionsMutex.Lock()
	defer seriesVersionsMutex.Unlock()

	updateSeriesVersionsOnce()
	return getOSFromSeries(series)
}

func getOSFromSeries(series string) (os.OSType, error) {
	if _, ok := ubuntuSeries[series]; ok {
		return os.Ubuntu, nil
	}
	if _, ok := centosSeries[series]; ok {
		return os.CentOS, nil
	}
	if _, ok := opensuseSeries[series]; ok {
		return os.OpenSUSE, nil
	}
	if _, ok := kubernetesSeries[series]; ok {
		return os.Kubernetes, nil
	}
	if series == genericLinuxSeries {
		return os.GenericLinux, nil
	}
	for _, val := range windowsVersions {
		if val == series {
			return os.Windows, nil
		}
	}
	for _, val := range windowsNanoVersions {
		if val == series {
			return os.Windows, nil
		}
	}
	for _, val := range macOSXSeries {
		if val == series {
			return os.OSX, nil
		}
	}

	return os.Unknown, errors.Trace(unknownOSForSeriesError(series))
}

// GetOSFromSeriesWithBaseOS will return the operating system based
// on the series and optional os base that is passed to it
func GetOSFromSeriesWithBaseOS(series, baseOS string) (os.OSType, error) {
	if series == "" {
		return os.Unknown, errors.NotValidf("series %q", series)
	}
	osType, err := getOSFromSeriesWithBaseOS(series, baseOS)
	if err == nil {
		return osType, nil
	}

	seriesVersionsMutex.Lock()
	defer seriesVersionsMutex.Unlock()

	updateSeriesVersionsOnce()
	return getOSFromSeriesWithBaseOS(series, baseOS)
}

func getOSFromSeriesWithBaseOS(series, baseOS string) (os.OSType, error) {
	switch strings.ToLower(baseOS) {
	case "centos":
		if _, ok := centosSeries["centos"+series]; ok {
			return os.CentOS, nil
		}
	}
	return getOSFromSeries(series)
}

var (
	seriesVersionsMutex sync.Mutex
)

// SeriesVersion returns the version for the specified series.
func SeriesVersion(series string) (string, error) {
	if series == "" {
		return "", errors.Trace(unknownSeriesVersionError(""))
	}
	seriesVersionsMutex.Lock()
	defer seriesVersionsMutex.Unlock()
	if vers, ok := seriesVersions[series]; ok {
		return vers, nil
	}
	updateSeriesVersionsOnce()
	if vers, ok := seriesVersions[series]; ok {
		return vers, nil
	}

	return "", errors.Trace(unknownSeriesVersionError(series))
}

// UbuntuSeriesVersion returns the ubuntu version for the specified series.
func UbuntuSeriesVersion(series string) (string, error) {
	if series == "" {
		return "", errors.Trace(unknownSeriesVersionError(""))
	}
	seriesVersionsMutex.Lock()
	defer seriesVersionsMutex.Unlock()
	if vers, ok := ubuntuSeries[series]; ok {
		return vers.Version, nil
	}
	updateSeriesVersionsOnce()
	if vers, ok := ubuntuSeries[series]; ok {
		return vers.Version, nil
	}

	return "", errors.Trace(unknownSeriesVersionError(series))
}

// VersionSeries returns the series (e.g.trusty) for the specified version (e.g. 14.04).
func VersionSeries(version string) (string, error) {
	if version == "" {
		return "", errors.Trace(unknownVersionSeriesError(""))
	}
	seriesVersionsMutex.Lock()
	defer seriesVersionsMutex.Unlock()
	if series, ok := versionSeries[version]; ok {
		return series, nil
	}
	updateSeriesVersionsOnce()
	if series, ok := versionSeries[version]; ok {
		return series, nil
	}
	return "", errors.Trace(unknownVersionSeriesError(version))
}

// WindowsVersionSeries returns the series (eg: win2012r2) for the specified version
// (eg: Windows Server 2012 R2 Standard)
func WindowsVersionSeries(version string) (string, error) {
	if version == "" {
		return "", errors.Trace(unknownVersionSeriesError(""))
	}
	for _, val := range windowsVersionMatchOrder {
		if strings.HasPrefix(version, val) {
			return windowsVersions[val], nil
		}
	}
	return "", errors.Trace(unknownVersionSeriesError(""))
}

// CentOSVersionSeries validates that the supplied series (eg: centos7)
// is supported.
func CentOSVersionSeries(version string) (string, error) {
	if version == "" {
		return "", errors.Trace(unknownVersionSeriesError(""))
	}
	if series, ok := centosSeries[version]; ok {
		return series, nil
	}
	return "", errors.Trace(unknownVersionSeriesError(""))

}

// SupportedLts are the current supported LTS series in ascending order.
func SupportedLts() []string {
	seriesVersionsMutex.Lock()
	defer seriesVersionsMutex.Unlock()
	updateSeriesVersionsOnce()

	versions := []string{}
	for _, version := range ubuntuSeries {
		if !version.LTS || !version.Supported {
			continue
		}
		versions = append(versions, version.Version)
	}
	sort.Strings(versions)
	sorted := []string{}
	for _, v := range versions {
		sorted = append(sorted, versionSeries[v])
	}
	return sorted
}

// latestLtsSeries is used to ensure we only do
// the work to determine the latest lts series once.
var latestLtsSeries string

// LatestLts returns the Latest LTS Series found in distro-info
func LatestLts() string {
	if latestLtsSeries != "" {
		return latestLtsSeries
	}

	seriesVersionsMutex.Lock()
	defer seriesVersionsMutex.Unlock()
	updateSeriesVersionsOnce()

	var latest string
	for k, version := range ubuntuSeries {
		if !version.LTS || !version.Supported {
			continue
		}
		if version.Version > ubuntuSeries[latest].Version {
			latest = k
		}
	}

	latestLtsSeries = latest
	return latest
}

// SetLatestLtsForTesting is provided to allow tests to override the lts series
// used and decouple the tests from the host by avoiding calling out to
// distro-info.  It returns the previous setting so that it may be set back to
// the original value by the caller.
func SetLatestLtsForTesting(series string) string {
	old := latestLtsSeries
	latestLtsSeries = series
	return old
}

func updateVersionSeries() {
	versionSeries = reverseSeriesVersion()
}

// reverseSeriesVersion returns reverse of seriesVersion map,
// keyed on versions with series as values.
func reverseSeriesVersion() map[string]string {
	reverse := make(map[string]string, len(seriesVersions))
	for k, v := range seriesVersions {
		reverse[v] = k
	}
	return reverse
}

// SupportedSeries returns the series on which we can run Juju workloads.
func SupportedSeries() []string {
	seriesVersionsMutex.Lock()
	defer seriesVersionsMutex.Unlock()
	updateSeriesVersionsOnce()
	var series []string
	for s := range seriesVersions {
		series = append(series, s)
	}
	return series
}

type namedSeriesVersion struct {
	Name          string
	SeriesVersion SeriesVersionInfo
	Version       float64
}

func ubuntuSeriesSortedByVersion() []namedSeriesVersion {
	seriesVersionsMutex.Lock()
	defer seriesVersionsMutex.Unlock()
	updateSeriesVersionsOnce()

	s := make([]namedSeriesVersion, 0, len(ubuntuSeries))
	for name, series := range ubuntuSeries {
		ver, err := strconv.ParseFloat(series.Version, 10)
		if err != nil {
			ver = math.MaxFloat64
		}

		s = append(s, namedSeriesVersion{
			Name:          name,
			SeriesVersion: series,
			Version:       ver,
		})
	}

	sort.Slice(s, func(i, j int) bool {
		if s[i].Version > s[j].Version {
			return true
		}
		if s[i].Version < s[j].Version {
			return false
		}
		return s[i].Name < s[j].Name
	})

	return s
}

// SupportedJujuControllerSeries returns a slice of juju supported series that
// target a controller (bootstrapping).
//
// The series are sorted in release version.
//   - focal (20.04)
//   - bionic (18.04)
//   - xenial (16.04)
//
// Anything not supported is left out.
func SupportedJujuControllerSeries() []string {
	s := ubuntuSeriesSortedByVersion()

	var series []string
	for _, version := range s {
		if !version.SeriesVersion.Supported {
			continue
		}
		series = append(series, version.Name)
	}
	return series
}

// SupportedJujuWorkloadSeries returns a slice of juju supported series that
// target a workload (deploying a charm).
//
// The series are sorted in ubuntu release version, anything that isn't
// ubuntu release is then sorted by name.
//   - focal (20.04)
//   - bionic (18.04)
//   - xenial (16.04)
//   - centos7
//   - centos8
//   - genericlinux
//   - kubernetes
//   - opensuseleap
//   - win10
//   - win2008r2
//
// Anything not supported is left out.
func SupportedJujuWorkloadSeries() []string {
	var result []string
	// Ensure that ubuntu series are first!
	result = append(result, SupportedJujuControllerSeries()...)

	// Then perform the surgery on the non ubuntu series.
	seriesVersionsMutex.Lock()
	defer seriesVersionsMutex.Unlock()
	updateSeriesVersionsOnce()

	var series []string
	for s, version := range nonUbuntuSeries {
		if !version.Supported {
			continue
		}
		series = append(series, s)
	}
	sort.Strings(series)
	return append(result, series...)
}

// SupportedJujuSeries returns a slice of juju supported series that also
// target a workload.
func SupportedJujuSeries() []string {
	return SupportedJujuWorkloadSeries()
}

// ESMSupportedJujuSeries returns a slice of just juju extended security
// maintenance supported ubuntu series.
// The series are sorted in release version.
//   - jammy (22.04)
//   - focal (20.04)
//   - bionic (18.04)
//   - xenial (16.04)
//
// Anything not supported is left out.
func ESMSupportedJujuSeries() []string {
	s := ubuntuSeriesSortedByVersion()

	var series []string
	for _, version := range s {
		if !version.SeriesVersion.ESMSupported {
			continue
		}
		series = append(series, version.Name)
	}
	return series
}

// OSSupportedSeries returns the series of the specified OS on which we
// can run Juju workloads.
func OSSupportedSeries(os os.OSType) []string {
	var osSeries []string
	for _, series := range SupportedSeries() {
		seriesOS, err := GetOSFromSeries(series)
		if err != nil || seriesOS != os {
			continue
		}
		osSeries = append(osSeries, series)
	}
	return osSeries
}

// UpdateSeriesVersions forces an update of the series versions by querying
// distro-info if possible.
func UpdateSeriesVersions() error {
	seriesVersionsMutex.Lock()
	defer seriesVersionsMutex.Unlock()

	err := updateLocalSeriesVersions()
	if err != nil {
		return err
	}
	updateVersionSeries()
	latestLtsSeries = ""
	return nil
}

var updatedseriesVersions bool

func updateSeriesVersionsOnce() {
	if !updatedseriesVersions {
		if err := updateLocalSeriesVersions(); err != nil {
			logger.Warningf("failed to update distro info: %v", err)
		}
		updateVersionSeries()
		updatedseriesVersions = true
	}
}
