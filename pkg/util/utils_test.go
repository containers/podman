package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	sliceData = []string{"one", "two", "three", "four"}
)

func TestStringInSlice(t *testing.T) {
	// string is in the slice
	assert.True(t, StringInSlice("one", sliceData))
	// string is not in the slice
	assert.False(t, StringInSlice("five", sliceData))
	// string is not in empty slice
	assert.False(t, StringInSlice("one", []string{}))
}

func TestGetImageConfigUser(t *testing.T) {
	validUser, err := GetImageConfig([]string{"USER valid"})
	require.Nil(t, err)
	assert.Equal(t, validUser.User, "valid")

	validUser2, err := GetImageConfig([]string{"USER test_user_2"})
	require.Nil(t, err)
	assert.Equal(t, validUser2.User, "test_user_2")

	_, err = GetImageConfig([]string{"USER "})
	assert.NotNil(t, err)
}

func TestGetImageConfigExpose(t *testing.T) {
	validPortNoProto, err := GetImageConfig([]string{"EXPOSE 80"})
	require.Nil(t, err)
	_, exists := validPortNoProto.ExposedPorts["80/tcp"]
	assert.True(t, exists)

	validPortTCP, err := GetImageConfig([]string{"EXPOSE 80/tcp"})
	require.Nil(t, err)
	_, exists = validPortTCP.ExposedPorts["80/tcp"]
	assert.True(t, exists)

	validPortUDP, err := GetImageConfig([]string{"EXPOSE 80/udp"})
	require.Nil(t, err)
	_, exists = validPortUDP.ExposedPorts["80/udp"]
	assert.True(t, exists)

	_, err = GetImageConfig([]string{"EXPOSE 99999"})
	assert.NotNil(t, err)

	_, err = GetImageConfig([]string{"EXPOSE 80/notaproto"})
	assert.NotNil(t, err)

	_, err = GetImageConfig([]string{"EXPOSE "})
	assert.NotNil(t, err)

	_, err = GetImageConfig([]string{"EXPOSE thisisnotanumber"})
	assert.NotNil(t, err)
}

func TestGetImageConfigEnv(t *testing.T) {
	validEnvNoValue, err := GetImageConfig([]string{"ENV key"})
	require.Nil(t, err)
	assert.True(t, StringInSlice("key=", validEnvNoValue.Env))

	validEnvBareEquals, err := GetImageConfig([]string{"ENV key="})
	require.Nil(t, err)
	assert.True(t, StringInSlice("key=", validEnvBareEquals.Env))

	validEnvKeyValue, err := GetImageConfig([]string{"ENV key=value"})
	require.Nil(t, err)
	assert.True(t, StringInSlice("key=value", validEnvKeyValue.Env))

	validEnvKeyMultiEntryValue, err := GetImageConfig([]string{`ENV key="value1 value2"`})
	require.Nil(t, err)
	assert.True(t, StringInSlice("key=value1 value2", validEnvKeyMultiEntryValue.Env))

	_, err = GetImageConfig([]string{"ENV "})
	assert.NotNil(t, err)
}

func TestGetImageConfigEntrypoint(t *testing.T) {
	binShEntrypoint, err := GetImageConfig([]string{"ENTRYPOINT /bin/bash"})
	require.Nil(t, err)
	require.Equal(t, 3, len(binShEntrypoint.Entrypoint))
	assert.Equal(t, binShEntrypoint.Entrypoint[0], "/bin/sh")
	assert.Equal(t, binShEntrypoint.Entrypoint[1], "-c")
	assert.Equal(t, binShEntrypoint.Entrypoint[2], "/bin/bash")

	entrypointWithSpaces, err := GetImageConfig([]string{"ENTRYPOINT ls -al"})
	require.Nil(t, err)
	require.Equal(t, 3, len(entrypointWithSpaces.Entrypoint))
	assert.Equal(t, entrypointWithSpaces.Entrypoint[0], "/bin/sh")
	assert.Equal(t, entrypointWithSpaces.Entrypoint[1], "-c")
	assert.Equal(t, entrypointWithSpaces.Entrypoint[2], "ls -al")

	jsonArrayEntrypoint, err := GetImageConfig([]string{`ENTRYPOINT ["ls", "-al"]`})
	require.Nil(t, err)
	require.Equal(t, 2, len(jsonArrayEntrypoint.Entrypoint))
	assert.Equal(t, jsonArrayEntrypoint.Entrypoint[0], "ls")
	assert.Equal(t, jsonArrayEntrypoint.Entrypoint[1], "-al")

	emptyEntrypoint, err := GetImageConfig([]string{"ENTRYPOINT "})
	require.Nil(t, err)
	assert.Equal(t, 0, len(emptyEntrypoint.Entrypoint))

	emptyEntrypointArray, err := GetImageConfig([]string{"ENTRYPOINT []"})
	require.Nil(t, err)
	assert.Equal(t, 0, len(emptyEntrypointArray.Entrypoint))
}

func TestGetImageConfigCmd(t *testing.T) {
	binShCmd, err := GetImageConfig([]string{"CMD /bin/bash"})
	require.Nil(t, err)
	require.Equal(t, 3, len(binShCmd.Cmd))
	assert.Equal(t, binShCmd.Cmd[0], "/bin/sh")
	assert.Equal(t, binShCmd.Cmd[1], "-c")
	assert.Equal(t, binShCmd.Cmd[2], "/bin/bash")

	cmdWithSpaces, err := GetImageConfig([]string{"CMD ls -al"})
	require.Nil(t, err)
	require.Equal(t, 3, len(cmdWithSpaces.Cmd))
	assert.Equal(t, cmdWithSpaces.Cmd[0], "/bin/sh")
	assert.Equal(t, cmdWithSpaces.Cmd[1], "-c")
	assert.Equal(t, cmdWithSpaces.Cmd[2], "ls -al")

	jsonArrayCmd, err := GetImageConfig([]string{`CMD ["ls", "-al"]`})
	require.Nil(t, err)
	require.Equal(t, 2, len(jsonArrayCmd.Cmd))
	assert.Equal(t, jsonArrayCmd.Cmd[0], "ls")
	assert.Equal(t, jsonArrayCmd.Cmd[1], "-al")

	emptyCmd, err := GetImageConfig([]string{"CMD "})
	require.Nil(t, err)
	require.Equal(t, 2, len(emptyCmd.Cmd))
	assert.Equal(t, emptyCmd.Cmd[0], "/bin/sh")
	assert.Equal(t, emptyCmd.Cmd[1], "-c")

	blankCmd, err := GetImageConfig([]string{"CMD []"})
	require.Nil(t, err)
	assert.Equal(t, 0, len(blankCmd.Cmd))
}

func TestGetImageConfigVolume(t *testing.T) {
	oneLenJSONArrayVol, err := GetImageConfig([]string{`VOLUME ["/test1"]`})
	require.Nil(t, err)
	_, exists := oneLenJSONArrayVol.Volumes["/test1"]
	assert.True(t, exists)
	assert.Equal(t, 1, len(oneLenJSONArrayVol.Volumes))

	twoLenJSONArrayVol, err := GetImageConfig([]string{`VOLUME ["/test1", "/test2"]`})
	require.Nil(t, err)
	assert.Equal(t, 2, len(twoLenJSONArrayVol.Volumes))
	_, exists = twoLenJSONArrayVol.Volumes["/test1"]
	assert.True(t, exists)
	_, exists = twoLenJSONArrayVol.Volumes["/test2"]
	assert.True(t, exists)

	oneLenVol, err := GetImageConfig([]string{"VOLUME /test1"})
	require.Nil(t, err)
	_, exists = oneLenVol.Volumes["/test1"]
	assert.True(t, exists)
	assert.Equal(t, 1, len(oneLenVol.Volumes))

	twoLenVol, err := GetImageConfig([]string{"VOLUME /test1 /test2"})
	require.Nil(t, err)
	assert.Equal(t, 2, len(twoLenVol.Volumes))
	_, exists = twoLenVol.Volumes["/test1"]
	assert.True(t, exists)
	_, exists = twoLenVol.Volumes["/test2"]
	assert.True(t, exists)

	_, err = GetImageConfig([]string{"VOLUME []"})
	assert.NotNil(t, err)

	_, err = GetImageConfig([]string{"VOLUME "})
	assert.NotNil(t, err)

	_, err = GetImageConfig([]string{`VOLUME [""]`})
	assert.NotNil(t, err)
}

func TestGetImageConfigWorkdir(t *testing.T) {
	singleWorkdir, err := GetImageConfig([]string{"WORKDIR /testdir"})
	require.Nil(t, err)
	assert.Equal(t, singleWorkdir.WorkingDir, "/testdir")

	twoWorkdirs, err := GetImageConfig([]string{"WORKDIR /testdir", "WORKDIR a"})
	require.Nil(t, err)
	assert.Equal(t, twoWorkdirs.WorkingDir, "/testdir/a")

	_, err = GetImageConfig([]string{"WORKDIR "})
	assert.NotNil(t, err)
}

func TestGetImageConfigLabel(t *testing.T) {
	labelNoQuotes, err := GetImageConfig([]string{"LABEL key1=value1"})
	require.Nil(t, err)
	assert.Equal(t, labelNoQuotes.Labels["key1"], "value1")

	labelWithQuotes, err := GetImageConfig([]string{`LABEL "key 1"="value 2"`})
	require.Nil(t, err)
	assert.Equal(t, labelWithQuotes.Labels["key 1"], "value 2")

	labelNoValue, err := GetImageConfig([]string{"LABEL key="})
	require.Nil(t, err)
	contents, exists := labelNoValue.Labels["key"]
	assert.True(t, exists)
	assert.Equal(t, contents, "")

	_, err = GetImageConfig([]string{"LABEL key"})
	assert.NotNil(t, err)

	_, err = GetImageConfig([]string{"LABEL "})
	assert.NotNil(t, err)
}

func TestGetImageConfigOnBuild(t *testing.T) {
	onBuildOne, err := GetImageConfig([]string{"ONBUILD ADD /testdir1"})
	require.Nil(t, err)
	require.Equal(t, 1, len(onBuildOne.OnBuild))
	assert.Equal(t, onBuildOne.OnBuild[0], "ADD /testdir1")

	onBuildTwo, err := GetImageConfig([]string{"ONBUILD ADD /testdir1", "ONBUILD ADD /testdir2"})
	require.Nil(t, err)
	require.Equal(t, 2, len(onBuildTwo.OnBuild))
	assert.Equal(t, onBuildTwo.OnBuild[0], "ADD /testdir1")
	assert.Equal(t, onBuildTwo.OnBuild[1], "ADD /testdir2")

	_, err = GetImageConfig([]string{"ONBUILD "})
	assert.NotNil(t, err)
}

func TestGetImageConfigMisc(t *testing.T) {
	_, err := GetImageConfig([]string{""})
	assert.NotNil(t, err)

	_, err = GetImageConfig([]string{"USER"})
	assert.NotNil(t, err)

	_, err = GetImageConfig([]string{"BADINST testvalue"})
	assert.NotNil(t, err)
}

func TestValidateSysctls(t *testing.T) {
	strSlice := []string{"net.core.test1=4", "kernel.msgmax=2"}
	result, _ := ValidateSysctls(strSlice)
	assert.Equal(t, result["net.core.test1"], "4")
}

func TestValidateSysctlBadSysctl(t *testing.T) {
	strSlice := []string{"BLAU=BLUE", "GELB^YELLOW"}
	_, err := ValidateSysctls(strSlice)
	assert.Error(t, err)
}

func TestCoresToPeriodAndQuota(t *testing.T) {
	cores := 1.0
	expectedPeriod := DefaultCPUPeriod
	expectedQuota := int64(DefaultCPUPeriod)

	actualPeriod, actualQuota := CoresToPeriodAndQuota(cores)
	assert.Equal(t, actualPeriod, expectedPeriod, "Period does not match")
	assert.Equal(t, actualQuota, expectedQuota, "Quota does not match")
}

func TestPeriodAndQuotaToCores(t *testing.T) {
	var (
		period        uint64 = 100000
		quota         int64  = 50000
		expectedCores        = 0.5
	)

	assert.Equal(t, PeriodAndQuotaToCores(period, quota), expectedCores)
}
