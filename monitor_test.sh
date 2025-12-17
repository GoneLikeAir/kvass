#!/bin/bash

echo "开始监控系统资源使用情况..."
echo "测试开始时间: $(date)"
echo "========================================"

# 记录测试前的系统状态
echo "测试前系统状态:"
echo "内存使用情况:"
vm_stat | head -10
echo ""
echo "CPU 使用情况:"
top -l 1 | head -10
echo ""

# 启动测试并监控资源使用
echo "开始运行极限压力测试..."
echo "========================================"

# 在后台启动测试
go test -race -run TestMetricInfo_ExtremeStressTest -v ./pkg/scrape/ &
TEST_PID=$!

# 监控测试进程的资源使用
echo "测试进程 PID: $TEST_PID"
echo "监控测试过程中的资源使用..."
echo "========================================"

# 每30秒检查一次资源使用
for i in {1..10}; do
    if kill -0 $TEST_PID 2>/dev/null; then
        echo "第 $i 次检查 ($(date)):"
        ps -p $TEST_PID -o pid,ppid,pcpu,pmem,time,vsz,rss,comm
        echo ""
        sleep 30
    else
        echo "测试进程已结束"
        break
    fi
done

# 等待测试完成
wait $TEST_PID
TEST_EXIT_CODE=$?

echo "========================================"
echo "测试完成时间: $(date)"
echo "测试退出代码: $TEST_EXIT_CODE"

# 记录测试后的系统状态
echo ""
echo "测试后系统状态:"
echo "内存使用情况:"
vm_stat | head -10
echo ""
echo "CPU 使用情况:"
top -l 1 | head -10