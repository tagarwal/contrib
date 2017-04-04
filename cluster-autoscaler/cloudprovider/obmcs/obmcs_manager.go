package obmcs

import (
	"io"
	"strconv"
	"github.com/golang/glog"
	"gopkg.in/gcfg.v1"
	"net/http"
	"net/url"
	"os"
	"fmt"
	"os/exec"
	"io/ioutil"
	"path/filepath"
	"strings"
	"encoding/json"
)

type ObmcsManager struct {
	obmscNodeGroups  []*ObmcsNodeGroup
	bmkopsServiceHost string
}

type Config struct {
	bmkopsServiceHost string `sca00bhw:8080`
}

// CreateObmcsManager constructs the manager
func CreateObmcsManager(configReader io.Reader) (*ObmcsManager, error){
	var bmkopsServiceHost string
	if configReader != nil {
		var cfg Config
		if err := gcfg.ReadInto(&cfg, configReader); err != nil {
			glog.Errorf("Could not read config: %v", err)
		}
		bmkopsServiceHost = cfg.bmkopsServiceHost
	} else {
		bmkopsServiceHost = "sca00bhw:8080"
	}

	manager := &ObmcsManager{
		obmscNodeGroups: make([]*ObmcsNodeGroup,0),
		bmkopsServiceHost: bmkopsServiceHost,
	}

	return manager, nil
}

func (obmcsManager *ObmcsManager)registerNodeGroup(nodegroup *ObmcsNodeGroup) error {
	obmcsManager.obmscNodeGroups = append(obmcsManager.obmscNodeGroups,nodegroup)
	return nil
}

func cpbmcsk8skeys(clustername string, home string) {
	// first copy the bmcsk8skeys to their proper location
	files := [2]string{filepath.Join("/bmkops/bmcsk8skeys", clustername + ".pem"),
						filepath.Join("/bmkops/bmcsk8skeys", clustername + "-key.pem")}

	certsdir := filepath.Join(home,".bmkops/",clustername,"certs")
	err := os.MkdirAll(certsdir,777)

	if err != nil {
		glog.Fatalf("failed to make certsdir: %v", err)
	}

	for _, file := range files {
		data,err := ioutil.ReadFile(file)
		if err != nil {
			glog.Fatal(err)
		}
		certFilePath := filepath.Join(certsdir,filepath.Base(file))
		err = ioutil.WriteFile(certFilePath, data, 0644)
		if err != nil {
			glog.Fatal(err)
		}
	}
}

func (obmcsManager *ObmcsManager)ScaleUpNode(obmcsNodeGroup *ObmcsNodeGroup, size int) error {

	clustername := os.Getenv("CLUSTER_NAME")
	compartmentname := os.Getenv("OBMCS_COMPARTMENT_NAME")
	home := os.Getenv("HOME")

	cpbmcsk8skeys(clustername,home)
	priv_key := filepath.Join("/bmkops","keys","priv-key")
	pub_key := filepath.Join("/bmkops","keys","pub-key")

	cmd := exec.Command("bmkops","cluster","-c","/.oraclebmc/config",
						"-C",compartmentname,"add-minion",
						"-n", clustername,
						"-w", strconv.FormatInt(int64(size),10),
						"-e", "VM.Standard1.4",
						"-k", pub_key,
						"-K", priv_key)
	glog.Infof("running the following command for adding minion", strings.Join(cmd.Args, " "))
	stdout,err := cmd.StdoutPipe()
	if err != nil {
		glog.Fatalf("Failed to get pipe to stdout %s\n",err)
	}
	stderr,err := cmd.StderrPipe()
	if err != nil {
		glog.Fatalf("Failed to get pipe to stderr %s\n",err)
	}
	err = cmd.Start()
	if err != nil {
		fmt.Printf("Failed to run bmkops %s", err)
		glog.Fatalf("Failed to run bmkops add-minion due to the following error %v", err)
	}

	go io.Copy(os.Stdout, stdout)
	go io.Copy(os.Stderr, stderr)
	err = cmd.Wait()
	return err
}

func (obmcsManager *ObmcsManager)GetOBMCSClusterSize() (int, error) {
	cluster_name := os.Getenv("CLUSTER_NAME")
	compartment_name := os.Getenv("OBMCS_COMPARTMENT_NAME")

	cmd := exec.Command("bmkops","cluster","-c","/.oraclebmc/config",
		"-C",compartment_name,"get",
		"-n", cluster_name,
		"-o", "JSON")
	glog.Infof("running the following command for getting cluster info \n", strings.Join(cmd.Args, " "))
	fmt.Printf("running the following command for getting cluster info , %v", strings.Join(cmd.Args," "))

	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("Failed to run bmkops %s", err)
		glog.Fatalf("Failed to run bmkops cluster get %v", err)
	}

	no_of_minions,err := returnNoOfMinions(output)
	glog.V(4).Infof("The number of minions in %s cluster is %d", cluster_name, no_of_minions)
	return no_of_minions, err
}

func returnNoOfMinions(cmd_output []byte) (int,error){

	var ret int = 0
	var f interface{}
	err := json.Unmarshal(cmd_output, &f)
	if err != nil {
		glog.Fatalf("Error while unmarshalling json data from %s, error is %v",cmd_output,err)
	}

	m := f.(map[string]interface{})
	//find the length of minions
	if val, ok := m["minions"]; ok {
		val := val.([]interface{})
		ret = len(val)
	} else {
		ret = 0
	}

	return ret, err
}

func (obmcs_manager *ObmcsManager) DeleteInstance(nodeGroupId string, instanceName string) error {
	cluster_name := os.Getenv("CLUSTER_NAME")
	compartment_name := os.Getenv("OBMCS_COMPARTMENT_NAME")

	cmd := exec.Command("bmkops","cluster","-c","/.oraclebmc/config",
		"-C",compartment_name,"rm-minion",
		"-n", cluster_name,
		"-m", instanceName,
		"-y")
	glog.V(4).Infof("running the following command for removing minion", strings.Join(cmd.Args, " "))
	fmt.Printf("running the following command for removing minion %v", strings.Join(cmd.Args," "))

	err := cmd.Run()
	if err != nil {
		fmt.Printf("Failed to run bmkops %s", err)
		glog.Fatalf("Failed to run bmkops rm-minion due to the following error %v", err)
	}

	return err
}

func (obmcsManager *ObmcsManager)ScaleUpNodeForHosted(obmcsNodegroup *ObmcsNodeGroup, size int64) error {
	// first get the cluster information
	// get the ip of master
	// get the node names for now , later just get the number of nodes to add
	// get the user who wants to add (at least for hosted boxes)
	// call bmkops addminion or hostedaddminion
	// for now just hard code

	serviceHost := os.Getenv("OBMCS_AUTOSCALE_SERVICE_HOST")
	servicePort := os.Getenv("OBMCS_AUTOSCALE_SERVICE_PORT")
	client := &http.Client{}
	urlString := "http://" + serviceHost + ":" + servicePort
	u,error := url.ParseRequestURI(urlString)
	if error != nil {
		glog.Fatal(error)
		return error
	}
	u.Path = "/hostedaddminion/targetCluster"
	v := url.Values{}
	v.Set("m","sca00kee")
	v.Set("w","sca00kef")
	v.Set("u","tagarwal")

	u.RawQuery = v.Encode()
	toSendURL := fmt.Sprintf("%v", u)

	r, error := http.NewRequest(http.MethodPost,toSendURL, nil)
	if error != nil {
		glog.Fatal(error)
		return error
	}
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Add("Content-Length", strconv.Itoa(len(v.Encode())))

	resp, err := client.Do(r)
	if err != nil{
		glog.Fatal(err)
		return err
	}

	defer resp.Body.Close()
	return nil
}