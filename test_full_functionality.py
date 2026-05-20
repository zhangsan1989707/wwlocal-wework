#!/usr/bin/env python3
"""
政务微信数据查询平台 - 全量功能测试脚本
测试完成后自动清理测试数据
"""

import requests
import json
import time
from datetime import datetime, timedelta
from typing import Dict, List, Any, Optional

BASE_URL = "http://47.109.85.168:5173/api/v1"
FRONTEND_URL = "http://47.109.85.168:5173"
TEST_USERNAME = "root"
TEST_PASSWORD = "Sfpy5NN;e"

class TestResult:
    def __init__(self):
        self.tests: List[Dict[str, Any]] = []
        self.passed = 0
        self.failed = 0
        self.test_data_created: List[Dict[str, Any]] = []

    def add_pass(self, name: str, details: str = ""):
        self.tests.append({
            "name": name,
            "status": "PASS",
            "details": details,
            "time": datetime.now().isoformat()
        })
        self.passed += 1
        print(f"✅ {name}")

    def add_fail(self, name: str, details: str = ""):
        self.tests.append({
            "name": name,
            "status": "FAIL",
            "details": details,
            "time": datetime.now().isoformat()
        })
        self.failed += 1
        print(f"❌ {name}: {details}")

    def record_test_data(self, data_type: str, data: Any):
        self.test_data_created.append({
            "type": data_type,
            "data": data,
            "created_at": datetime.now().isoformat()
        })

    def summary(self):
        total = self.passed + self.failed
        print("\n" + "="*60)
        print(f"测试结果: {self.passed}/{total} 通过")
        print(f"测试数据记录: {len(self.test_data_created)} 条")
        print("="*60)
        return {
            "total": total,
            "passed": self.passed,
            "failed": self.failed,
            "tests": self.tests,
            "test_data_created": self.test_data_created
        }

def login(result: TestResult) -> Optional[str]:
    """测试登录功能"""
    print("\n📝 测试登录功能...")

    try:
        response = requests.post(
            f"{BASE_URL}/api/v1/auth/login",
            json={"username": TEST_USERNAME, "password": TEST_PASSWORD},
            timeout=10
        )
        data = response.json()

        if data.get("code") == 0 and data.get("data", {}).get("token"):
            token = data["data"]["token"]
            result.add_pass("登录功能", f"获取到token，用户名: {data['data'].get('username')}")
            return token
        else:
            result.add_fail("登录功能", f"响应: {data}")
            return None
    except Exception as e:
        result.add_fail("登录功能", str(e))
        return None

def get_headers(token: str) -> Dict[str, str]:
    return {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json"
    }

def test_health_check(result: TestResult):
    """测试健康检查"""
    print("\n🏥 测试健康检查...")

    try:
        response = requests.get(f"{BASE_URL}/health", timeout=5)
        data = response.json()

        if response.status_code == 200 and data.get("status") == "ok":
            checks = data.get("checks", {})
            result.add_pass("健康检查", f"状态: {data.get('status')}, 检查项: {len(checks)}")
        else:
            result.add_fail("健康检查", f"响应: {data}")
    except Exception as e:
        result.add_fail("健康检查", str(e))

def test_dashboard_overview(result: TestResult, token: str):
    """测试仪表板概览"""
    print("\n📊 测试仪表板概览...")

    try:
        response = requests.get(
            f"{BASE_URL}/api/v1/dashboard/overview",
            headers=get_headers(token),
            timeout=10
        )
        data = response.json()

        if data.get("code") == 0 and data.get("data"):
            overview = data["data"]
            kpis = overview.get("kpis", {})
            result.add_pass(
                "仪表板概览",
                f"最近同步: {kpis.get('recent_sync_count')}, "
                f"7天同步: {kpis.get('synced_7d_count')}, "
                f"活跃密钥天数: {kpis.get('active_key_days')}"
            )
            return overview
        else:
            result.add_fail("仪表板概览", f"响应: {data}")
            return None
    except Exception as e:
        result.add_fail("仪表板概览", str(e))
        return None

def test_log_features(result: TestResult, token: str):
    """测试获取功能列表"""
    print("\n📋 测试获取功能列表...")

    try:
        response = requests.get(
            f"{BASE_URL}/api/v1/logs/features",
            headers=get_headers(token),
            timeout=10
        )
        data = response.json()

        if data.get("code") == 0 and data.get("data"):
            features = data["data"]
            feature_ids = [f["feature_id"] for f in features if f.get("enabled")]
            result.add_pass("获取功能列表", f"总功能数: {len(features)}, 启用: {len(feature_ids)}")
            result.record_test_data("features", feature_ids)
            return features
        else:
            result.add_fail("获取功能列表", f"响应: {data}")
            return None
    except Exception as e:
        result.add_fail("获取功能列表", str(e))
        return None

def test_log_time_range(result: TestResult, token: str):
    """测试获取时间范围"""
    print("\n⏰ 测试获取时间范围...")

    try:
        response = requests.get(
            f"{BASE_URL}/api/v1/logs/time-range",
            headers=get_headers(token),
            timeout=10
        )
        data = response.json()

        if data.get("code") == 0:
            time_range = data.get("data", {})
            earliest = time_range.get("earliest")
            latest = time_range.get("latest")

            if earliest and latest:
                earliest_dt = datetime.fromtimestamp(earliest/1000)
                latest_dt = datetime.fromtimestamp(latest/1000)
                result.add_pass(
                    "获取时间范围",
                    f"最早: {earliest_dt.strftime('%Y-%m-%d')}, "
                    f"最新: {latest_dt.strftime('%Y-%m-%d')}"
                )
            else:
                result.add_pass("获取时间范围", "暂无数据")
            return time_range
        else:
            result.add_fail("获取时间范围", f"响应: {data}")
            return None
    except Exception as e:
        result.add_fail("获取时间范围", str(e))
        return None

def test_log_query(result: TestResult, token: str, features: List[Dict]):
    """测试日志查询"""
    print("\n🔍 测试日志查询...")

    if not features:
        result.add_fail("日志查询", "没有可用的功能配置")
        return None

    enabled_features = [f for f in features if f.get("enabled")]
    if not enabled_features:
        result.add_fail("日志查询", "没有启用的功能")
        return None

    feature_id = enabled_features[0]["feature_id"]
    now = int(datetime.now().timestamp() * 1000)
    start_time = now - 7 * 24 * 60 * 60 * 1000

    try:
        response = requests.post(
            f"{BASE_URL}/api/v1/logs/query",
            headers=get_headers(token),
            json={
                "feature_ids": [feature_id],
                "start_time": start_time,
                "end_time": now,
                "page": 1,
                "page_size": 10
            },
            timeout=15
        )
        data = response.json()

        if data.get("code") == 0:
            query_result = data.get("data", {})
            total = query_result.get("total", 0)
            result.add_pass("日志查询", f"功能ID: {feature_id}, 总数: {total}")
            result.record_test_data("log_query", {
                "feature_id": feature_id,
                "start_time": start_time,
                "end_time": now
            })
            return query_result
        else:
            result.add_fail("日志查询", f"响应: {data}")
            return None
    except Exception as e:
        result.add_fail("日志查询", str(e))
        return None

def test_field_paths(result: TestResult, token: str):
    """测试获取字段路径"""
    print("\n🗂️ 测试获取字段路径...")

    try:
        response = requests.get(
            f"{BASE_URL}/api/v1/logs/field-paths",
            headers=get_headers(token),
            timeout=10
        )
        data = response.json()

        if data.get("code") == 0 and data.get("data"):
            fields = data["data"]
            result.add_pass("获取字段路径", f"字段数量: {len(fields)}")
            return fields
        else:
            result.add_fail("获取字段路径", f"响应: {data}")
            return None
    except Exception as e:
        result.add_fail("获取字段路径", str(e))
        return None

def test_sync_status(result: TestResult, token: str):
    """测试同步状态"""
    print("\n🔄 测试同步状态...")

    try:
        response = requests.get(
            f"{BASE_URL}/api/v1/logs/sync/status",
            headers=get_headers(token),
            timeout=10
        )
        data = response.json()

        if data.get("code") == 0:
            status = data.get("data", {})
            running = status.get("running", False)
            result.add_pass("同步状态", f"运行中: {running}")
            return status
        else:
            result.add_fail("同步状态", f"响应: {data}")
            return None
    except Exception as e:
        result.add_fail("同步状态", str(e))
        return None

def test_key_management(result: TestResult, token: str):
    """测试密钥管理"""
    print("\n🔑 测试密钥管理...")

    try:
        response = requests.get(
            f"{BASE_URL}/api/v1/keys",
            headers=get_headers(token),
            timeout=10
        )
        data = response.json()

        if data.get("code") == 0 and data.get("data"):
            keys = data["data"]
            active_key = next((k for k in keys if k.get("is_active")), None)
            result.add_pass(
                "密钥列表",
                f"总数: {len(keys)}, 激活版本: {active_key.get('version') if active_key else '无'}"
            )
            return keys
        else:
            result.add_fail("密钥列表", f"响应: {data}")
            return None
    except Exception as e:
        result.add_fail("密钥列表", str(e))
        return None

def test_scheduler_status(result: TestResult, token: str):
    """测试调度器状态"""
    print("\n⏱️ 测试调度器状态...")

    try:
        response = requests.get(
            f"{BASE_URL}/api/v1/scheduler/status",
            headers=get_headers(token),
            timeout=10
        )
        data = response.json()

        if data.get("code") == 0:
            status = data.get("data", {})
            running = status.get("running", False)
            interval = status.get("interval", "N/A")
            result.add_pass("调度器状态", f"运行中: {running}, 间隔: {interval}")
            return status
        else:
            result.add_fail("调度器状态", f"响应: {data}")
            return None
    except Exception as e:
        result.add_fail("调度器状态", str(e))
        return None

def test_contacts_list(result: TestResult, token: str):
    """测试通讯录列表"""
    print("\n👥 测试通讯录列表...")

    try:
        response = requests.get(
            f"{BASE_URL}/api/v1/contacts",
            headers=get_headers(token),
            params={"page": 1, "page_size": 10},
            timeout=10
        )
        data = response.json()

        if data.get("code") == 0:
            contacts_data = data.get("data", {})
            total = contacts_data.get("total", 0)
            result.add_pass("通讯录列表", f"总数: {total}")
            return contacts_data
        else:
            result.add_fail("通讯录列表", f"响应: {data}")
            return None
    except Exception as e:
        result.add_fail("通讯录列表", str(e))
        return None

def test_contacts_departments(result: TestResult, token: str):
    """测试获取部门列表"""
    print("\n🏢 测试部门列表...")

    try:
        response = requests.get(
            f"{BASE_URL}/api/v1/contacts/departments",
            headers=get_headers(token),
            timeout=10
        )
        data = response.json()

        if data.get("code") == 0 and data.get("data"):
            depts = data["data"]
            result.add_pass("部门列表", f"部门数量: {len(depts)}")
            return depts
        else:
            result.add_fail("部门列表", f"响应: {data}")
            return None
    except Exception as e:
        result.add_fail("部门列表", str(e))
        return None

def test_contacts_sync_status(result: TestResult, token: str):
    """测试通讯录同步状态"""
    print("\n🔄 测试通讯录同步状态...")

    try:
        response = requests.get(
            f"{BASE_URL}/api/v1/contacts/sync/status",
            headers=get_headers(token),
            timeout=10
        )
        data = response.json()

        if data.get("code") == 0:
            status = data.get("data", {})
            running = status.get("running", False)
            last_sync = status.get("last_sync", "N/A")
            result.add_pass("通讯录同步状态", f"运行中: {running}, 上次同步: {last_sync}")
            return status
        else:
            result.add_fail("通讯录同步状态", f"响应: {data}")
            return None
    except Exception as e:
        result.add_fail("通讯录同步状态", str(e))
        return None

def test_system_status(result: TestResult, token: str):
    """测试系统状态"""
    print("\n💻 测试系统状态...")

    try:
        response = requests.get(
            f"{BASE_URL}/api/v1/system/status",
            headers=get_headers(token),
            timeout=10
        )
        data = response.json()

        if data.get("code") == 0:
            status = data.get("data", {})
            db_connected = status.get("db_connected", False)
            uptime = status.get("uptime", "N/A")
            result.add_pass("系统状态", f"DB连接: {db_connected}, 运行时间: {uptime}")
            return status
        else:
            result.add_fail("系统状态", f"响应: {data}")
            return None
    except Exception as e:
        result.add_fail("系统状态", str(e))
        return None

def test_admin_oper_logs(result: TestResult, token: str):
    """测试管理员操作日志"""
    print("\n📜 测试管理员操作日志...")

    try:
        response = requests.get(
            f"{BASE_URL}/api/v1/admin-oper-logs",
            headers=get_headers(token),
            params={"page": 1, "page_size": 10},
            timeout=10
        )
        data = response.json()

        if data.get("code") == 0:
            logs_data = data.get("data", {})
            total = logs_data.get("total", 0)
            result.add_pass("管理员操作日志", f"总数: {total}")
            return logs_data
        else:
            result.add_fail("管理员操作日志", f"响应: {data}")
            return None
    except Exception as e:
        result.add_fail("管理员操作日志", str(e))
        return None

def test_sync_features(result: TestResult, token: str):
    """测试同步功能配置"""
    print("\n⚙️ 测试同步功能配置...")

    try:
        response = requests.get(
            f"{BASE_URL}/api/v1/sync-features",
            headers=get_headers(token),
            timeout=10
        )
        data = response.json()

        if data.get("code") == 0 and data.get("data"):
            features = data["data"]
            enabled = len([f for f in features if f.get("enabled")])
            result.add_pass("同步功能配置", f"总数: {len(features)}, 启用: {enabled}")
            return features
        else:
            result.add_fail("同步功能配置", f"响应: {data}")
            return None
    except Exception as e:
        result.add_fail("同步功能配置", str(e))
        return None

def test_operation_logs(result: TestResult, token: str):
    """测试操作日志"""
    print("\n📝 测试操作日志...")

    try:
        response = requests.get(
            f"{BASE_URL}/api/v1/operation-logs",
            headers=get_headers(token),
            params={"page": 1, "page_size": 10},
            timeout=10
        )
        data = response.json()

        if data.get("code") == 0:
            logs_data = data.get("data", {})
            total = logs_data.get("total", 0)
            result.add_pass("操作日志", f"总数: {total}")
            return logs_data
        else:
            result.add_fail("操作日志", f"响应: {data}")
            return None
    except Exception as e:
        result.add_fail("操作日志", str(e))
        return None

def test_inactive_users(result: TestResult, token: str):
    """测试不活跃用户"""
    print("\n👤 测试不活跃用户...")

    try:
        response = requests.get(
            f"{BASE_URL}/api/v1/dashboard/inactive-users",
            headers=get_headers(token),
            params={"page": 1, "page_size": 10, "min_inactive_days": 7},
            timeout=10
        )
        data = response.json()

        if data.get("code") == 0:
            users_data = data.get("data", {})
            total = users_data.get("total_contacts", 0)
            inactive = users_data.get("inactive_count", 0)
            result.add_pass("不活跃用户", f"总用户: {total}, 不活跃: {inactive}")
            return users_data
        else:
            result.add_fail("不活跃用户", f"响应: {data}")
            return None
    except Exception as e:
        result.add_fail("不活跃用户", str(e))
        return None

def test_tasks(result: TestResult, token: str):
    """测试任务列表"""
    print("\n📋 测试任务列表...")

    try:
        response = requests.get(
            f"{BASE_URL}/api/v1/tasks",
            headers=get_headers(token),
            timeout=10
        )
        data = response.json()

        if data.get("code") == 0:
            tasks = data.get("data", [])
            result.add_pass("任务列表", f"任务数: {len(tasks)}")
            return tasks
        else:
            result.add_fail("任务列表", f"响应: {data}")
            return None
    except Exception as e:
        result.add_fail("任务列表", str(e))
        return None

def run_all_tests():
    """运行所有测试"""
    print("="*60)
    print("政务微信数据查询平台 - 全量功能测试")
    print("="*60)
    print(f"后端地址: {BASE_URL}")
    print(f"前端地址: {FRONTEND_URL}")
    print(f"测试时间: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("="*60)

    result = TestResult()

    token = login(result)
    if not token:
        print("\n❌ 无法获取认证token，测试终止")
        return result.summary()

    test_health_check(result)

    features = test_log_features(result, token)

    test_dashboard_overview(result, token)

    test_log_time_range(result, token)

    test_field_paths(result, token)

    test_log_query(result, token, features if features else [])

    test_sync_status(result, token)

    test_key_management(result, token)

    test_scheduler_status(result, token)

    test_contacts_list(result, token)

    test_contacts_departments(result, token)

    test_contacts_sync_status(result, token)

    test_system_status(result, token)

    test_admin_oper_logs(result, token)

    test_sync_features(result, token)

    test_operation_logs(result, token)

    test_inactive_users(result, token)

    test_tasks(result, token)

    return result.summary()

if __name__ == "__main__":
    summary = run_all_tests()

    print("\n📊 测试数据记录:")
    if summary.get("test_data_created"):
        for item in summary["test_data_created"]:
            print(f"  - 类型: {item['type']}, 数据: {item['data']}")
    else:
        print("  无测试数据创建（仅做查询操作）")

    print("\n✅ 功能测试完成!")
    print("💡 说明: 本次测试主要进行只读查询操作，未创建需要清理的测试数据")
