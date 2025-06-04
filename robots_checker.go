package main

import (
	"github.com/temoto/robotstxt"
)

// RobotsChecker is a struct that checks if a path is allowed by robots.txt rules
type RobotsChecker struct {
	robotsData *robotstxt.RobotsData
}

// LoadRobots loads the robots.txt content into the RobotsChecker
func (rc *RobotsChecker) LoadRobots(pageContent string) error {
	robots, err := robotstxt.FromString(pageContent)
	if err != nil {
		return err
	}
	rc.robotsData = robots
	return nil
}

// IsAllowed checks if a given path is allowed for a specific user agent
func (rc *RobotsChecker) IsAllowed(path, userAgent string) bool {
	return rc.robotsData.TestAgent(path, userAgent)
}

// NewRobotsChecker creates a new RobotsChecker instance and loads the robots.txt content
func NewRobotsChecker(robotsTxt string) (*RobotsChecker, error) {
	rc := &RobotsChecker{}
	err := rc.LoadRobots(robotsTxt)
	if err != nil {
		return nil, err // or handle the error as needed
	}
	return rc, nil
}
