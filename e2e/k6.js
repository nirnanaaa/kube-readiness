import http from "k6/http";
import { check, sleep } from "k6";
import { Counter } from "k6/metrics";

export let CounterErrors = new Counter("Errors");

export let options = {
  vus: 10,
  duration: "180s",
  thresholds: {
    "Errors": [ { threshold: "count == 0", abortOnFail: true } ]
  }
};

export default function() {
  check(http.get(`http://${__ENV.APP_LB_DNS}`), {
    "status is 200 (OK)": (r) => r.status == 200
  }) || CounterErrors.add(true)
};
