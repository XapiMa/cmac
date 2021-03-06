package prof

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/xapima/conps/pkg/util"
)

const (
	OPEN   = uint(1)
	ACCESS = uint(1 << iota)
	BOTH   = OPEN | ACCESS

	DENY  = int(iota)
	ALLOW = int(iota)
)

var (
	permMap = map[uint]string{
		OPEN:   "o",
		ACCESS: "a",
		BOTH:   "ao",
	}
)

// {exeString:{targetPath:perm}}
type DenyProf map[string]map[string]uint
type AllowProf map[string]map[string]uint

type ProfApi struct {
	readFlag     int
	nowExeString string
	Deny         DenyProf
	Allow        AllowProf
}

func NewProfApi(profPath string) (*ProfApi, error) {
	deny := make(DenyProf)
	allow := make(AllowProf)
	p := &ProfApi{
		Deny:  deny,
		Allow: allow,
	}
	if err := p.readProf(profPath); err != nil {
		return nil, util.ErrorWrapFunc(err)
	}
	return p, nil
}

func (p *ProfApi) readProf(profPath string) error {
	f, err := os.Open(profPath)
	defer f.Close()
	if err != nil {
		return util.ErrorWrapFunc(err)
	}

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {

		if err := p.parseProfLine(scanner.Text()); err != nil {
			logrus.Error(util.ErrorWrapFunc(err))
		}
	}

	logrus.Debugf("allow: %v", p.Allow)
	logrus.Debugf("deny: %v", p.Deny)

	if err := p.checkProf(); err != nil {
		return util.ErrorWrapFunc(err)
	}

	return nil
}

func (p *ProfApi) parseProfLine(line string) error {
	logrus.Debugf("line: %v", line)
	switch {
	case strings.HasPrefix(strings.TrimSpace(line), "#"):
		break
	case strings.HasPrefix(line, "deny"):
		rawExeList := strings.TrimSpace((strings.TrimPrefix(line, "deny")))
		exeList := parseList(rawExeList)
		exeListString := strings.Join(exeList, ",")
		p.nowExeString = exeListString
		p.readFlag = DENY
		logrus.Debugf("exeListString: %v", exeListString)
	case strings.HasPrefix(line, "allow"):
		rawExeList := strings.TrimSpace((strings.TrimPrefix(line, "allow")))
		exeList := parseList(rawExeList)
		exeListString := strings.Join(exeList, ",")
		p.nowExeString = exeListString
		p.readFlag = ALLOW
		logrus.Debugf("exeListString: %v", exeListString)
	default:
		trimedSpace := strings.TrimSpace(line)
		if trimedSpace == "" {
			break
		}
		if !strings.HasPrefix(trimedSpace, "-") {
			return util.ErrorWrapFunc(fmt.Errorf("unknown line type: %v", line))
		}
		parts := strings.Split(trimedSpace[1:], ":")
		if len(parts) != 2 {
			return util.ErrorWrapFunc(fmt.Errorf("unknown line type: %v", line))
		}
		switch p.readFlag {
		case DENY:
			path := strings.Trim(strings.TrimSpace(parts[0]), "\"")
			switch strings.TrimSpace(parts[1]) {
			case "o":
				if _, ok := p.Deny[p.nowExeString]; !ok {
					p.Deny[p.nowExeString] = make(map[string]uint)
				}
				p.Deny[p.nowExeString][path] = OPEN
			case "a":
				if _, ok := p.Deny[p.nowExeString]; !ok {
					p.Deny[p.nowExeString] = make(map[string]uint)
				}
				p.Deny[p.nowExeString][path] = ACCESS
			case "oa", "ao":

				if _, ok := p.Deny[p.nowExeString]; !ok {
					p.Deny[p.nowExeString] = make(map[string]uint)
				}
				p.Deny[p.nowExeString][path] = BOTH
			default:
				return util.ErrorWrapFunc(fmt.Errorf("unkown permission: %v", parts[1]))
			}
		case ALLOW:
			path := strings.Trim(strings.TrimSpace(parts[0]), "\"")
			switch strings.TrimSpace(parts[1]) {
			case "o":
				if _, ok := p.Allow[p.nowExeString]; !ok {
					p.Allow[p.nowExeString] = make(map[string]uint)
				}
				p.Allow[p.nowExeString][path] = OPEN
			case "a":
				if _, ok := p.Allow[p.nowExeString]; !ok {
					p.Allow[p.nowExeString] = make(map[string]uint)
				}
				p.Allow[p.nowExeString][path] = ACCESS
			case "oa", "ao":

				if _, ok := p.Allow[p.nowExeString]; !ok {
					p.Allow[p.nowExeString] = make(map[string]uint)
				}
				p.Allow[p.nowExeString][path] = BOTH
			default:
				return util.ErrorWrapFunc(fmt.Errorf("unkown permission: %v", parts[1]))
			}
		}
	}
	return nil
}

func parseList(line string) []string {
	output := []string{}
	tsline := strings.TrimSpace(line)
	tkline := strings.Trim(tsline, "[]")
	parts := strings.Split(tkline, ",")
	for _, item := range parts {
		tsitem := strings.TrimSpace(item)
		tqitem := strings.Trim(tsitem, "\"")
		if tqitem != "" {
			output = append(output, filepath.Clean(tqitem))
		}
	}
	return output
}

func (p *ProfApi) checkProf() error {
	ng := []string{}
	for path, dm := range p.Deny {
		if am, ok := p.Allow[path]; ok {
			for dpath, dperm := range dm {
				for apath, aperm := range am {
					if dpath == apath && dperm&aperm != 0 {
						ng = append(ng, fmt.Sprintf("exe list : %s, target path: %s, perm: %s", path, dpath, permMap[dperm&aperm]))
					}
				}
			}
		}
	}
	if len(ng) != 0 {
		return fmt.Errorf("same target defined at deny rule and allow rule:\n\t- %v", strings.Join(ng, "\n\t-"))
	}
	return nil
}
