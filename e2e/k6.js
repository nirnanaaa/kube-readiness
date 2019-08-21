import http from "k6/http";
import { check, sleep } from "k6";
import { Counter } from "k6/metrics";

let CounterErrors = new Counter("Errors");

export let options = {
  vus: 10,
  duration: "30s",
  thresholds: {
    "Errors": [ { threshold: "count == 0" } ]
  }
};

export default function() {
  const res = http.get(`http://${__ENV.ECHOSERVER_LB_DNS}`)

  CounterErrors.add(check(res, {
    "status is 200 (OK)": (r) => r.status == 200
  }));

  sleep(1);
};
