import http from 'k6/http';
import { check, fail, sleep } from 'k6';

function intFromEnv(name, defaultValue) {
  const raw = __ENV[name];
  if (!raw) {
    return defaultValue;
  }

  const parsed = parseInt(raw, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : defaultValue;
}

function floatFromEnv(name, defaultValue) {
  const raw = __ENV[name];
  if (!raw) {
    return defaultValue;
  }

  const parsed = Number(raw);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : defaultValue;
}

const BASE_URL = (__ENV.BASE_URL || 'http://localhost:4433').replace(/\/$/, '');
const RUN_ID = __ENV.RUN_ID || String(Date.now());
const DEFAULT_PASSWORD = __ENV.K6_TEST_PASSWORD || 'K6pass!1234';

const FULL_START_VUS = intFromEnv('FULL_START_VUS', 1);
const FULL_TARGET_VUS = intFromEnv('FULL_TARGET_VUS', 8);
const FULL_SLEEP_SECONDS = floatFromEnv('FULL_SLEEP_SECONDS', 0.2);

const FULL_RAMP_UP = __ENV.FULL_RAMP_UP || '1m';
const FULL_STEADY = __ENV.FULL_STEADY || '3m';
const FULL_RAMP_DOWN = __ENV.FULL_RAMP_DOWN || '1m';

const SETUP_MERCHANT_EMAIL =
  __ENV.FULL_MERCHANT_EMAIL || `k6-merchant-${RUN_ID}@example.com`;
const SETUP_MERCHANT_NAME = __ENV.FULL_MERCHANT_NAME || `k6-merchant-${RUN_ID}`;
const SETUP_STORE_NAME = __ENV.FULL_STORE_NAME || `K6 Store ${RUN_ID}`;
const SETUP_BUSINESS_LICENSE =
  __ENV.FULL_BUSINESS_LICENSE || `K6-LICENSE-${RUN_ID}`;

export const options = {
  scenarios: {
    full_flow: {
      executor: 'ramping-vus',
      startVUs: FULL_START_VUS,
      stages: [
        { duration: FULL_RAMP_UP, target: FULL_TARGET_VUS },
        { duration: FULL_STEADY, target: FULL_TARGET_VUS },
        { duration: FULL_RAMP_DOWN, target: 0 },
      ],
      gracefulRampDown: '30s',
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.05'],
    http_req_duration: ['p(95)<1500'],
  },
};

function parseJSON(res) {
  try {
    return res.json();
  } catch (error) {
    return null;
  }
}

function truncate(text, maxLength) {
  if (!text) {
    return '';
  }

  if (text.length <= maxLength) {
    return text;
  }

  return `${text.slice(0, maxLength)}...`;
}

function assertStatus(res, expectedStatus, label) {
  const ok = check(res, {
    [`${label} status is ${expectedStatus}`]: (r) => r.status === expectedStatus,
  });

  if (!ok) {
    fail(
      `${label} failed with status=${res.status} body=${truncate(
        res.body,
        500,
      )}`,
    );
  }
}

function assertStatuses(res, allowedStatus, label) {
  const ok = check(res, {
    [`${label} status is ${allowedStatus.join('/')}`]: (r) =>
      allowedStatus.indexOf(r.status) >= 0,
  });

  if (!ok) {
    fail(
      `${label} failed with status=${res.status} body=${truncate(
        res.body,
        500,
      )}`,
    );
  }
}

function jsonParams(name, token) {
  const headers = {
    'Content-Type': 'application/json',
  };

  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  return {
    headers: headers,
    tags: { name: name },
  };
}

function post(path, payload, name, token) {
  return http.post(`${BASE_URL}${path}`, JSON.stringify(payload), jsonParams(name, token));
}

function put(path, payload, name, token) {
  return http.put(`${BASE_URL}${path}`, JSON.stringify(payload), jsonParams(name, token));
}

function get(path, name, token) {
  return http.get(`${BASE_URL}${path}`, jsonParams(name, token));
}

function del(path, name, token) {
  return http.del(`${BASE_URL}${path}`, null, jsonParams(name, token));
}

function logout(refreshToken, requestName) {
  const res = post(
    '/auth/logout',
    {
      refresh_token: refreshToken,
    },
    requestName,
  );
  assertStatus(res, 200, `${requestName} logout`);
}

function tryLogout(refreshToken, requestName) {
  if (!refreshToken) {
    return;
  }

  const res = post(
    '/auth/logout',
    {
      refresh_token: refreshToken,
    },
    requestName,
  );

  if (res.status !== 200) {
    console.error(
      `${requestName} best-effort logout failed with status=${res.status} body=${truncate(
        res.body,
        500,
      )}`,
    );
  }
}

function login(email, password, requestName) {
  const res = post('/auth/login', { email: email, password: password }, requestName);
  assertStatus(res, 200, `${requestName} login`);

  const body = parseJSON(res);
  const data = body && body.data ? body.data : null;
  const accessToken = data && data.access_token ? data.access_token : '';
  const refreshToken = data && data.refresh_token ? data.refresh_token : '';
  const userID = data && data.user && data.user.id ? data.user.id : '';

  if (!accessToken || !refreshToken || !userID) {
    fail(`${requestName} login response missing token/user id: ${truncate(res.body, 500)}`);
  }

  return {
    token: accessToken,
    refreshToken: refreshToken,
    userID: userID,
  };
}

export function setup() {
  const registerRes = post(
    '/auth/register/merchant',
    {
      name: SETUP_MERCHANT_NAME,
      email: SETUP_MERCHANT_EMAIL,
      password: DEFAULT_PASSWORD,
      store_name: SETUP_STORE_NAME,
      business_license: SETUP_BUSINESS_LICENSE,
    },
    'full_setup_register_merchant',
  );
  assertStatuses(registerRes, [201, 409], 'setup register merchant');

  const merchantLogin = login(
    SETUP_MERCHANT_EMAIL,
    DEFAULT_PASSWORD,
    'full_setup_login_merchant',
  );
  logout(merchantLogin.refreshToken, 'full_setup_logout_merchant');

  return {
    merchantEmail: SETUP_MERCHANT_EMAIL,
    merchantPassword: DEFAULT_PASSWORD,
    merchantID: merchantLogin.userID,
  };
}

export default function (setupData) {
  const userSuffix = `${RUN_ID}-vu${__VU}-iter${__ITER}`;
  const userEmail = `k6-user-${userSuffix}@example.com`;
  const userPassword = DEFAULT_PASSWORD;

  const registerUserRes = post(
    '/auth/register/user',
    {
      name: `k6-user-${userSuffix}`,
      email: userEmail,
      password: userPassword,
    },
    'full_register_user',
  );
  assertStatuses(registerUserRes, [201, 409], 'register user');

  let userLogin = null;
  let merchantLogin = null;

  try {
    userLogin = login(userEmail, userPassword, 'full_login_user');
    merchantLogin = login(
      setupData.merchantEmail,
      setupData.merchantPassword,
      'full_login_merchant',
    );

    const userProfileRes = get('/user/profile', 'full_get_profile', userLogin.token);
    assertStatus(userProfileRes, 200, 'get user profile');
    check(parseJSON(userProfileRes), {
      'profile has id': (body) => body && body.data && body.data.id,
      'profile has email': (body) => body && body.data && body.data.email,
    });

    const userLocationRes = post(
      '/api/v1/locations/user',
      {
        label: `home-${userSuffix}`,
        full_address: 'No. 1, Test Road, Taipei',
        latitude: 25.033,
        longitude: 121.5654,
        is_primary: true,
        is_active: true,
      },
      'full_create_user_location',
      userLogin.token,
    );
    assertStatus(userLocationRes, 201, 'create user location');
    const userLocationBody = parseJSON(userLocationRes);
    const userLocationID =
      userLocationBody && userLocationBody.data && userLocationBody.data.id
        ? userLocationBody.data.id
        : '';
    if (!userLocationID) {
      fail(`create user location response missing id: ${truncate(userLocationRes.body, 500)}`);
    }

    const updateUserLocationRes = put(
      `/api/v1/locations/user/${userLocationID}`,
      {
        label: `home-updated-${userSuffix}`,
        is_active: true,
      },
      'full_update_user_location',
      userLogin.token,
    );
    assertStatus(updateUserLocationRes, 200, 'update user location');

    const deleteUserLocationRes = del(
      `/api/v1/locations/user/${userLocationID}`,
      'full_delete_user_location',
      userLogin.token,
    );
    assertStatus(deleteUserLocationRes, 200, 'delete user location');

    const registerDeviceRes = post(
      '/api/v1/devices',
      {
        fcm_token: `fcm-token-${userSuffix}`,
        device_id: `device-${userSuffix}`,
        platform: 'android',
      },
      'full_register_device',
      userLogin.token,
    );
    assertStatus(registerDeviceRes, 201, 'register device');
    const registerDeviceBody = parseJSON(registerDeviceRes);
    const deviceID =
      registerDeviceBody && registerDeviceBody.data && registerDeviceBody.data.id
        ? registerDeviceBody.data.id
        : '';
    if (!deviceID) {
      fail(`register device response missing id: ${truncate(registerDeviceRes.body, 500)}`);
    }

    const getDevicesRes = get('/api/v1/devices', 'full_get_devices', userLogin.token);
    assertStatus(getDevicesRes, 200, 'get devices');

    const updateDeviceTokenRes = put(
      `/api/v1/devices/${deviceID}/token`,
      {
        fcm_token: `fcm-token-updated-${userSuffix}`,
      },
      'full_update_device_token',
      userLogin.token,
    );
    assertStatus(updateDeviceTokenRes, 200, 'update device token');

    const deactivateDeviceRes = del(
      `/api/v1/devices/${deviceID}`,
      'full_deactivate_device',
      userLogin.token,
    );
    assertStatus(deactivateDeviceRes, 200, 'deactivate device');

    const subscribeRes = post(
      '/api/v1/subscriptions',
      {
        merchant_id: setupData.merchantID,
      },
      'full_subscribe',
      userLogin.token,
    );
    assertStatus(subscribeRes, 201, 'subscribe merchant');

    const listSubscriptionsRes = get(
      '/api/v1/subscriptions',
      'full_list_subscriptions',
      userLogin.token,
    );
    assertStatus(listSubscriptionsRes, 200, 'list subscriptions');

    const unsubscribeRes = del(
      `/api/v1/subscriptions/${setupData.merchantID}`,
      'full_unsubscribe',
      userLogin.token,
    );
    assertStatus(unsubscribeRes, 200, 'unsubscribe merchant');

    const merchantLocationRes = post(
      '/api/v1/locations/merchant',
      {
        label: `merchant-location-${userSuffix}`,
        full_address: 'No. 100, Merchant Street, Taipei',
        latitude: 25.0478,
        longitude: 121.5319,
        is_primary: false,
        is_active: true,
      },
      'full_create_merchant_location',
      merchantLogin.token,
    );
    assertStatus(merchantLocationRes, 201, 'create merchant location');
    const merchantLocationBody = parseJSON(merchantLocationRes);
    const merchantLocationID =
      merchantLocationBody && merchantLocationBody.data && merchantLocationBody.data.id
        ? merchantLocationBody.data.id
        : '';
    if (!merchantLocationID) {
      fail(
        `create merchant location response missing id: ${truncate(
          merchantLocationRes.body,
          500,
        )}`,
      );
    }

    const publishNotificationRes = post(
      '/api/v1/notifications',
      {
        location_data: {
          location_name: `spot-${userSuffix}`,
          full_address: 'No. 100, Merchant Street, Taipei',
          latitude: 25.0478,
          longitude: 121.5319,
        },
        hint_message: 'k6 full test',
      },
      'full_publish_notification',
      merchantLogin.token,
    );
    assertStatus(publishNotificationRes, 201, 'publish notification');

    const historyRes = get(
      '/api/v1/notifications?limit=10&offset=0',
      'full_get_notification_history',
      merchantLogin.token,
    );
    assertStatus(historyRes, 200, 'get notification history');

    const deleteMerchantLocationRes = del(
      `/api/v1/locations/merchant/${merchantLocationID}`,
      'full_delete_merchant_location',
      merchantLogin.token,
    );
    assertStatus(deleteMerchantLocationRes, 200, 'delete merchant location');

    sleep(FULL_SLEEP_SECONDS);
  } finally {
    if (userLogin) {
      tryLogout(userLogin.refreshToken, 'full_finally_logout_user');
    }

    if (merchantLogin) {
      tryLogout(merchantLogin.refreshToken, 'full_finally_logout_merchant');
    }
  }
}
