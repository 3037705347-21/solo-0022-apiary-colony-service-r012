package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"example.com/apiary-colony-service/internal/domain"
	"example.com/apiary-colony-service/internal/status"
)

const (
	demoApiaryID = "apiary-north"
	demoHiveID   = "hive-01"
)

func seedApiary(c *client, id, name, district, keeper string) error {
	_, err := c.expect("POST", "/apiaries", map[string]any{
		"id":       id,
		"name":     name,
		"district": district,
		"keeper":   keeper,
	}, 201, `"id":"`+id+`"`)
	return err
}

func seedHive(c *client, apiaryID, hiveID, model string) error {
	_, err := c.expect("POST", "/apiaries/"+apiaryID+"/hives", map[string]any{
		"id":    hiveID,
		"model": model,
	}, 201, `"state":"colonized"`)
	return err
}

func seedDefault(c *client) error {
	if err := seedApiary(c, demoApiaryID, "北岭蜂场", "北岭片区", "周岚"); err != nil {
		return err
	}
	return seedHive(c, demoApiaryID, demoHiveID, "langstroth")
}

func checkRegisterApiary(c *client) error {
	if _, err := c.expect("POST", "/apiaries", map[string]any{
		"id": "apiary-north", "name": "北岭蜂场", "district": "北岭片区", "keeper": "周岚",
	}, 201, `"id":"apiary-north"`, `"name":"北岭蜂场"`); err != nil {
		return err
	}
	if _, err := c.expect("GET", "/apiaries/apiary-north", nil, 200,
		`"district":"北岭片区"`, `"keeper":"周岚"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/apiaries", map[string]any{
		"id": "apiary-north", "name": "重复蜂场", "district": "北岭片区", "keeper": "周岚",
	}, 409, `"code":"conflict"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/apiaries", map[string]any{
		"id": "apiary-south", "name": "", "district": "南坡片区", "keeper": "李澈",
	}, 400, `"code":"invalid_request"`); err != nil {
		return err
	}
	return nil
}

func checkHiveInstall(c *client) error {
	if err := seedApiary(c, demoApiaryID, "北岭蜂场", "北岭片区", "周岚"); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/apiaries/apiary-north/hives", map[string]any{
		"id": "hive-01", "model": "langstroth",
	}, 201, `"state":"colonized"`, `"apiaryId":"apiary-north"`); err != nil {
		return err
	}
	if _, err := c.expect("GET", "/hives/hive-01", nil, 200, `"model":"langstroth"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/apiaries/apiary-north/hives", map[string]any{
		"id": "hive-01", "model": "warre",
	}, 409, `"code":"conflict"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/apiaries/apiary-missing/hives", map[string]any{
		"id": "hive-09", "model": "warre",
	}, 404, `"code":"not_found"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/apiaries/apiary-north/hives", map[string]any{
		"id": "hive-02", "model": "dadant",
	}, 400, `"code":"invalid_request"`); err != nil {
		return err
	}
	return nil
}

func checkRecordInspection(c *client) error {
	if err := seedDefault(c); err != nil {
		return err
	}
	// 群势越界必须整体退回，不能夹到区间后保存。
	if _, err := c.expect("POST", "/hives/hive-01/inspections", map[string]any{
		"observedAt": "2026-04-10T09:00:00Z", "colonyStrength": 9, "broodPattern": "patchy",
		"temper": "calm", "miteCount": 3, "note": "",
	}, 400, `"code":"invalid_request"`, `"field":"colonyStrength"`); err != nil {
		return err
	}
	// 螨数越界同样整体退回。
	if _, err := c.expect("POST", "/hives/hive-01/inspections", map[string]any{
		"observedAt": "2026-04-10T09:00:00Z", "colonyStrength": 3, "broodPattern": "patchy",
		"temper": "calm", "miteCount": 61, "note": "",
	}, 400, `"code":"invalid_request"`, `"field":"miteCount"`); err != nil {
		return err
	}
	// 被拒绝的提交不能留下任何记录（也不能占用编号）。
	if reply, err := c.expect("GET", "/hives/hive-01/inspections", nil, 200, "[]"); err != nil {
		return err
	} else if strings.TrimSpace(reply.text()) != "[]" {
		return fmt.Errorf("被退回的巡检不应留下记录，实际 %s", reply.text())
	}
	// 合法量测：子脾带首尾空白，规范化后必须与回执、列表、概览保持同一份内容。
	first, err := c.expect("POST", "/hives/hive-01/inspections", map[string]any{
		"observedAt": "2026-04-10T09:00:00Z", "colonyStrength": 2, "broodPattern": " none ",
		"temper": "calm", "miteCount": 3, "note": "西侧巢门观察",
	}, 201, `"id":"inspection-000001"`, `"broodPattern":"none"`,
		`"concerns":["brood_gap","low_strength"]`)
	if err != nil {
		return err
	}
	firstRecord, err := decodeInspection(first.text())
	if err != nil {
		return err
	}
	if err := statusConcernsEqual(c, firstRecord, []string{"brood_gap", "low_strength"}); err != nil {
		return err
	}
	listed, err := c.expect("GET", "/hives/hive-01/inspections", nil, 200,
		`"id":"inspection-000001"`, `"note":"西侧巢门观察"`, `"broodPattern":"none"`,
		`"concerns":["brood_gap","low_strength"]`)
	if err != nil {
		return err
	}
	if err := listContainsExact(listed.text(), firstRecord); err != nil {
		return err
	}
	// 正常范围内但触发螨负荷与性情关注项的量测仍可正常提交。
	second, err := c.expect("POST", "/hives/hive-01/inspections", map[string]any{
		"observedAt": "2026-04-12T09:00:00Z", "colonyStrength": 3, "broodPattern": "patchy",
		"temper": "aggressive", "miteCount": 12, "note": "",
	}, 201, `"id":"inspection-000002"`, `"miteCount":12`,
		`"concerns":["mite_load","temper_risk"]`)
	if err != nil {
		return err
	}
	secondRecord, err := decodeInspection(second.text())
	if err != nil {
		return err
	}
	listed, err = c.call("GET", "/hives/hive-01/inspections", nil)
	if err != nil {
		return err
	}
	if err := listContainsExact(listed.text(), secondRecord); err != nil {
		return err
	}
	// 概览必须以最近一次巡检为准，且关注项与该次记录逐字一致。
	if err := statusConcernsEqual(c, secondRecord, []string{"mite_load", "temper_risk"}); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/state", map[string]any{"target": "retired"}, 200,
		`"to":"retired"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/inspections", map[string]any{
		"observedAt": "2026-04-13T09:00:00Z", "colonyStrength": 3, "broodPattern": "solid",
		"temper": "calm", "miteCount": 1, "note": "",
	}, 409, `"code":"invalid_state"`); err != nil {
		return err
	}
	return nil
}

func decodeInspection(body string) (domain.Inspection, error) {
	var record domain.Inspection
	if err := json.Unmarshal([]byte(body), &record); err != nil {
		return domain.Inspection{}, fmt.Errorf("解析巡检回执失败：%v（%s）", err, body)
	}
	return record, nil
}

// listContainsExact 断言巡检列表中逐字段存在与回执完全相同的记录。
func listContainsExact(body string, want domain.Inspection) error {
	var records []domain.Inspection
	if err := json.Unmarshal([]byte(body), &records); err != nil {
		return fmt.Errorf("解析巡检列表失败：%v（%s）", err, body)
	}
	for _, record := range records {
		if reflect.DeepEqual(record, want) {
			return nil
		}
	}
	return fmt.Errorf("巡检列表缺少与回执完全一致的记录 %+v，实际 %s", want, body)
}

// statusConcernsEqual 断言蜂场概览里该蜂箱的最近关注项与巡检记录同源一致，
// 且该蜂箱正是据此进入/离开 concernHives。
func statusConcernsEqual(c *client, want domain.Inspection, wantConcerns []string) error {
	reply, err := c.call("GET", "/apiaries/"+demoApiaryID+"/status?asOf=2026-04-12T00:00:00Z", nil)
	if err != nil {
		return err
	}
	var report status.Report
	if err := json.Unmarshal(reply.body, &report); err != nil {
		return fmt.Errorf("解析蜂场概览失败：%v（%s）", err, reply.text())
	}
	for _, summary := range report.Hives {
		if summary.HiveID != want.HiveID {
			continue
		}
		lastAt := summary.LastInspectionAt
		if lastAt == nil || !lastAt.Equal(want.ObservedAt) {
			return fmt.Errorf("概览最近巡检时间 %v 与记录 %v 不一致", lastAt, want.ObservedAt)
		}
		if !reflect.DeepEqual([]string(summary.LatestConcerns), wantConcerns) ||
			!reflect.DeepEqual([]string(want.Concerns), wantConcerns) {
			return fmt.Errorf("概览关注项 %v 与记录关注项 %v 不一致", summary.LatestConcerns, want.Concerns)
		}
		foundInConcernHives := false
		for _, id := range report.ConcernHives {
			if id == want.HiveID {
				foundInConcernHives = true
			}
		}
		if len(wantConcerns) > 0 && !foundInConcernHives {
			return fmt.Errorf("记录存在关注项 %v，但概览未把 %q 列入 concernHives", wantConcerns, want.HiveID)
		}
		return nil
	}
	return fmt.Errorf("概览中缺少蜂箱 %q：%s", want.HiveID, reply.text())
}

func checkReplaceQueen(c *client) error {
	if err := seedDefault(c); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/queens", map[string]any{
		"origin": "bred", "introducedAt": "2026-04-02T08:00:00Z", "reason": "aging",
	}, 201, `"queen-000001"`, `"markColor":"white"`, `"state":"laying"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/queens", map[string]any{
		"origin": "cells", "introducedAt": "2026-04-20T08:00:00Z", "reason": "aggression",
	}, 201, `"queen-000002"`, `"markColor":"white"`); err != nil {
		return err
	}
	if _, err := c.expect("GET", "/hives/hive-01/queens", nil, 200,
		`"queen-000001"`, `"superseded"`, `"queen-000002"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/queens", map[string]any{
		"origin": "imported", "introducedAt": "2026-05-01T08:00:00Z", "reason": "aging",
	}, 400, `"code":"invalid_request"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/state", map[string]any{"target": "retired"}, 200,
		`"to":"retired"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/queens", map[string]any{
		"origin": "bred", "introducedAt": "2026-05-02T08:00:00Z", "reason": "loss",
	}, 409, `"code":"invalid_state"`); err != nil {
		return err
	}
	return nil
}

func checkAdvanceHiveState(c *client) error {
	if err := seedDefault(c); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/state", map[string]any{"target": "production"}, 200,
		`"from":"colonized"`, `"to":"production"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/state", map[string]any{"target": "colonized"}, 409,
		`"code":"invalid_state"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/state", map[string]any{"target": "production"}, 409,
		`"code":"invalid_state"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/state", map[string]any{"target": "retired"}, 200,
		`"to":"retired"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/state", map[string]any{"target": "production"}, 409,
		`"code":"invalid_state"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-missing/state", map[string]any{"target": "production"}, 404,
		`"code":"not_found"`); err != nil {
		return err
	}
	return nil
}

func checkReadApiaryStatus(c *client) error {
	if err := seedApiary(c, demoApiaryID, "北岭蜂场", "北岭片区", "周岚"); err != nil {
		return err
	}
	for _, hiveID := range []string{"hive-01", "hive-02", "hive-03"} {
		if err := seedHive(c, demoApiaryID, hiveID, "langstroth"); err != nil {
			return err
		}
	}
	if _, err := c.expect("POST", "/hives/hive-02/state", map[string]any{"target": "production"}, 200,
		`"to":"production"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/inspections", map[string]any{
		"observedAt": "2026-04-10T09:00:00Z", "colonyStrength": 4, "broodPattern": "solid",
		"temper": "calm", "miteCount": 12, "note": "",
	}, 201, `"mite_load"`); err != nil {
		return err
	}
	report, err := c.expect("GET", "/apiaries/apiary-north/status?asOf=2026-04-12T00:00:00Z", nil, 200,
		`"hiveCount":3`, `"colonized":2`, `"production":1`, `"layingQueens":0`)
	if err != nil {
		return err
	}
	for _, fragment := range []string{
		`"needsInspection":["hive-02","hive-03"]`,
		`"concernHives":["hive-01"]`,
	} {
		if !strings.Contains(report.text(), fragment) {
			return fmt.Errorf("状态汇总缺少 %q：%s", fragment, report.text())
		}
	}
	if err := report.mustOrder(`"hiveId":"hive-01"`, `"hiveId":"hive-02"`); err != nil {
		return err
	}
	if _, err := c.expect("GET", "/apiaries/apiary-missing/status", nil, 404, `"code":"not_found"`); err != nil {
		return err
	}
	if _, err := c.expect("GET", "/apiaries/apiary-north/status?asOf=not-a-time", nil, 400,
		`"code":"invalid_request"`); err != nil {
		return err
	}
	return nil
}
