import http from 'k6/http';
import { check, fail, sleep } from 'k6';

function numberFromEnv(name, defaultValue) {
  const raw = __ENV[name];
  if (!raw) {
    return defaultValue;
  }

  const parsed = Number(raw);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : defaultValue;
}

function boolFromEnv(name, defaultValue) {
  const raw = __ENV[name];
  if (!raw) {
    return defaultValue;
  }

  return String(raw).toLowerCase() === 'true';
}

const BASE_URL = (__ENV.BASE_URL || 'http://localhost:4433').replace(/\/$/, '');
const SMOKE_DURATION = __ENV.SMOKE_DURATION || '1m';
const SMOKE_SETUP_TIMEOUT = __ENV.SMOKE_SETUP_TIMEOUT || '10m';
const SMOKE_SLEEP_SECONDS = numberFromEnv('SMOKE_SLEEP_SECONDS', 0);
const SMOKE_POOL_SIZE = Math.max(1, Math.floor(numberFromEnv('SMOKE_POOL_SIZE', 200)));
const SMOKE_RUN_ID = __ENV.SMOKE_RUN_ID || String(Date.now());
const SMOKE_PASSWORD = __ENV.SMOKE_PASSWORD || 'K6SmokePass1!';
const SMOKE_DO_LOGOUT = boolFromEnv('SMOKE_DO_LOGOUT', true);

export const options = {
  vus: numberFromEnv('SMOKE_VUS', 20),
  duration: SMOKE_DURATION,
  setupTimeout: SMOKE_SETUP_TIMEOUT,
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<800'],
    'http_req_duration{type:member_login}': ['p(95)<700'],
    'http_req_duration{type:merchant_login}': ['p(95)<700'],
  },
};

function parseJSON(res) {
  try {
    return res.json();
  } catch (error) {
    return null;
  }
}

function assertStatus(res, status, label) {
  const ok = check(res, {
    [`${label} status is ${status}`]: (r) => r.status === status,
  });

  if (!ok) {
    fail(`${label} failed with status=${res.status} body=${res.body}`);
  }
}

function assertStatuses(res, statuses, label) {
  const ok = check(res, {
    [`${label} status is ${statuses.join('/')}`]: (r) =>
      statuses.indexOf(r.status) >= 0,
  });

  if (!ok) {
    fail(`${label} failed with status=${res.status} body=${res.body}`);
  }
}

function buildMemberAccount(index) {
  return {
    name: `k6-smoke-member-${SMOKE_RUN_ID}-${index}`,
    email: `k6-smoke-member-${SMOKE_RUN_ID}-${index}@example.com`,
  };
}

function buildMerchantAccount(index) {
  return {
    name: `k6-smoke-merchant-${SMOKE_RUN_ID}-${index}`,
    email: `k6-smoke-merchant-${SMOKE_RUN_ID}-${index}@example.com`,
    storeName: `K6 Smoke Store ${SMOKE_RUN_ID}-${index}`,
    businessLicense: `K6-SMOKE-LICENSE-${SMOKE_RUN_ID}-${index}`,
  };
}

function registerMember(account) {
  const res = http.post(
    `${BASE_URL}/auth/register/user`,
    JSON.stringify({
      name: account.name,
      email: account.email,
      password: SMOKE_PASSWORD,
    }),
    {
      headers: {
        'Content-Type': 'application/json',
      },
      tags: {
        name: 'smoke_register_member',
        type: 'member_register',
      },
    },
  );
  assertStatuses(res, [201, 409], 'member register');
}

function registerMerchant(account) {
  const res = http.post(
    `${BASE_URL}/auth/register/merchant`,
    JSON.stringify({
      name: account.name,
      email: account.email,
      password: SMOKE_PASSWORD,
      store_name: account.storeName,
      business_license: account.businessLicense,
    }),
    {
      headers: {
        'Content-Type': 'application/json',
      },
      tags: {
        name: 'smoke_register_merchant',
        type: 'merchant_register',
      },
    },
  );
  assertStatuses(res, [201, 409], 'merchant register');
}

function login(email, nameTag, typeTag) {
  const res = http.post(
    `${BASE_URL}/auth/login`,
    JSON.stringify({
      email: email,
      password: SMOKE_PASSWORD,
    }),
    {
      headers: {
        'Content-Type': 'application/json',
      },
      tags: {
        name: nameTag,
        type: typeTag,
      },
    },
  );
  assertStatus(res, 200, `${typeTag} login`);

  const body = parseJSON(res);
  const data = body && body.data ? body.data : null;
  const accessToken = data && data.access_token ? data.access_token : '';
  const refreshToken = data && data.refresh_token ? data.refresh_token : '';
  if (!accessToken || !refreshToken) {
    fail(`${typeTag} login token is missing: ${res.body}`);
  }

  return {
    accessToken: accessToken,
    refreshToken: refreshToken,
  };
}

function logout(refreshToken, nameTag, typeTag) {
  const res = http.post(
    `${BASE_URL}/auth/logout`,
    JSON.stringify({
      refresh_token: refreshToken,
    }),
    {
      headers: {
        'Content-Type': 'application/json',
      },
      tags: {
        name: nameTag,
        type: typeTag,
      },
    },
  );
  assertStatus(res, 200, `${typeTag} logout`);
}

function pickRandom(list) {
  const index = Math.floor(Math.random() * list.length);
  return list[index];
}

export function setup() {
  const members = [];
  const merchants = [];

  for (let i = 0; i < SMOKE_POOL_SIZE; i += 1) {
    const member = buildMemberAccount(i);
    const merchant = buildMerchantAccount(i);

    registerMember(member);
    registerMerchant(merchant);

    members.push(member);
    merchants.push(merchant);
  }

  return {
    members: members,
    merchants: merchants,
  };
}

export default function (data) {
  const member = pickRandom(data.members);
  const merchant = pickRandom(data.merchants);

  const memberSession = login(member.email, 'smoke_login_member', 'member_login');
  const merchantSession = login(
    merchant.email,
    'smoke_login_merchant',
    'merchant_login',
  );

  if (SMOKE_DO_LOGOUT) {
    logout(memberSession.refreshToken, 'smoke_logout_member', 'member_logout');
    logout(
      merchantSession.refreshToken,
      'smoke_logout_merchant',
      'merchant_logout',
    );
  }

  if (SMOKE_SLEEP_SECONDS > 0) {
    sleep(SMOKE_SLEEP_SECONDS);
  }
}
