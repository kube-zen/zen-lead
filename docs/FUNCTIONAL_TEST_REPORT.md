# zen-lead Functional Test Report

**Test Date:** 2026-01-02 12:54:38 UTC
**Version:** 0.1.0-alpha-optimized
**Image Tag:** 0.1.0-alpha-optimized
**Cluster Context:** k3d-astesterole
**Namespace:** zen-lead-test
**Number of Failovers:** 50

## Test Summary

- ✅ zen-lead controller installed and ready
- ✅ Test deployment created with 3 replicas
- ✅ Leader service created: test-app-leader
- ✅ EndpointSlice created with endpoint: 10.42.0.23

## Failover Test

**Initial Leader Pod:** test-app-6f46c75fc7-hj226
**Failover Start Time:** 2026-01-02 12:55:10 UTC
**New Leader Pod:** test-app-6f46c75fc7-926w2
**Failover End Time:** 2026-01-02 12:55:12 UTC
**Downtime:** 1.851857417s

- ✅ Failover test completed successfully

## Multiple Failover Test (50 failovers)

**Failover 1:** 1.519755392s (from test-app-6f46c75fc7-926w2 to test-app-6f46c75fc7-ffsnp)
**Failover 2:** 1.923283858s (from test-app-6f46c75fc7-ffsnp to test-app-6f46c75fc7-pmps9)
**Failover 3:** 1.985691540s (from test-app-6f46c75fc7-pmps9 to test-app-6f46c75fc7-kprpr)
**Failover 4:** 1.781976080s (from test-app-6f46c75fc7-kprpr to test-app-6f46c75fc7-kjgl9)
**Failover 5:** 1.991953038s (from test-app-6f46c75fc7-kjgl9 to test-app-6f46c75fc7-ggr9w)
**Failover 10:** .953179509s (from test-app-6f46c75fc7-qvrbx to test-app-6f46c75fc7-8vdvz)
**Failover 20:** 1.228623407s (from test-app-6f46c75fc7-rz6v5 to test-app-6f46c75fc7-nlhm8)
**Failover 30:** 1.224001344s (from test-app-6f46c75fc7-bjxs2 to test-app-6f46c75fc7-smvkz)
**Failover 40:** .969601358s (from test-app-6f46c75fc7-l8j7m to test-app-6f46c75fc7-vvv2b)
**Failover 46:** .959875337s (from test-app-6f46c75fc7-l64vn to test-app-6f46c75fc7-hwrdn)
**Failover 47:** 1.226386486s (from test-app-6f46c75fc7-hwrdn to test-app-6f46c75fc7-pm7mp)
**Failover 48:** 1.697691575s (from test-app-6f46c75fc7-pm7mp to test-app-6f46c75fc7-jfw9x)
**Failover 49:** 1.178421494s (from test-app-6f46c75fc7-jfw9x to test-app-6f46c75fc7-4ww24)
**Failover 50:** 1.652599527s (from test-app-6f46c75fc7-4ww24 to test-app-6f46c75fc7-9fmkp)

**Statistics:**
- **Total Failovers:** 50
- **Successful:** 50
- **Failed:** 0
- **Success Rate:** 100.0%
- **Min Failover Time:** .899337781s
- **Max Failover Time:** 1.991953038s
- **Average Failover Time:** 1.207s
- **Total Test Duration:** ~1 minutes

## Test Conclusion

**Test completed at:** 2026-01-02 12:58:05 UTC
