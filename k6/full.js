import http from "k6/http";
import { check, fail, sleep } from "k6";

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

const BASE_URL = (__ENV.BASE_URL || "http://localhost:4433").replace(/\/$/, "");
const RUN_ID = __ENV.RUN_ID || String(Date.now());
const DEFAULT_PASSWORD = __ENV.K6_TEST_PASSWORD || "K6pass!1234";

const FULL_VUS = intFromEnv("FULL_VUS", 1);
const FULL_ITERATIONS = intFromEnv("FULL_ITERATIONS", 1);
const FULL_SLEEP_SECONDS = floatFromEnv("FULL_SLEEP_SECONDS", 0.2);

const SETUP_MERCHANT_EMAIL =
  __ENV.FULL_MERCHANT_EMAIL || `k6-merchant-${RUN_ID}@example.com`;
const SETUP_MERCHANT_NAME = __ENV.FULL_MERCHANT_NAME || `k6-merchant-${RUN_ID}`;
const SETUP_STORE_NAME = __ENV.FULL_STORE_NAME || `K6 Store ${RUN_ID}`;

export const options = {
  scenarios: {
    full_flow: {
      executor: "shared-iterations",
      vus: FULL_VUS,
      iterations: FULL_ITERATIONS,
      maxDuration: __ENV.FULL_MAX_DURATION || "5m",
    },
  },
  thresholds: {
    checks: ["rate==1"],
    http_req_failed: ["rate==0"],
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
    return "";
  }

  if (text.length <= maxLength) {
    return text;
  }

  return `${text.slice(0, maxLength)}...`;
}

function assertStatus(res, expectedStatus, label) {
  const ok = check(res, {
    [`${label} status is ${expectedStatus}`]: (r) =>
      r.status === expectedStatus,
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
    [`${label} status is ${allowedStatus.join("/")}`]: (r) =>
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
    "Content-Type": "application/json",
  };

  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  return {
    headers: headers,
    tags: { name: name },
  };
}

function requestParams(name, token) {
  const headers = {};

  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  return {
    headers: headers,
    tags: { name: name },
  };
}

function post(path, payload, name, token) {
  return http.post(
    `${BASE_URL}${path}`,
    JSON.stringify(payload),
    jsonParams(name, token),
  );
}

function put(path, payload, name, token) {
  return http.put(
    `${BASE_URL}${path}`,
    JSON.stringify(payload),
    jsonParams(name, token),
  );
}

function get(path, name, token) {
  return http.get(`${BASE_URL}${path}`, requestParams(name, token));
}

function del(path, name, token) {
  return http.del(`${BASE_URL}${path}`, null, jsonParams(name, token));
}

function patch(path, payload, name, token) {
  return http.patch(
    `${BASE_URL}${path}`,
    JSON.stringify(payload),
    jsonParams(name, token),
  );
}

function logout(refreshToken, requestName) {
  const res = post(
    "/auth/logout",
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
    "/auth/logout",
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
  const res = post(
    "/auth/login",
    { email: email, password: password },
    requestName,
  );
  assertStatus(res, 200, `${requestName} login`);

  const body = parseJSON(res);
  const data = body && body.data ? body.data : null;
  const accessToken = data && data.access_token ? data.access_token : "";
  const refreshToken = data && data.refresh_token ? data.refresh_token : "";
  const userID = data && data.user && data.user.id ? data.user.id : "";

  if (!accessToken || !refreshToken || !userID) {
    fail(
      `${requestName} login response missing token/user id: ${truncate(res.body, 500)}`,
    );
  }

  return {
    token: accessToken,
    refreshToken: refreshToken,
    userID: userID,
  };
}

function refresh(refreshToken, requestName) {
  const res = post(
    "/auth/refresh",
    { refresh_token: refreshToken },
    requestName,
  );
  assertStatus(res, 200, `${requestName} refresh`);

  const body = parseJSON(res);
  const data = body && body.data ? body.data : null;
  const accessToken = data && data.access_token ? data.access_token : "";
  const nextRefreshToken = data && data.refresh_token ? data.refresh_token : "";

  if (!accessToken || !nextRefreshToken) {
    fail(
      `${requestName} refresh response missing token: ${truncate(res.body, 500)}`,
    );
  }

  return {
    token: accessToken,
    refreshToken: nextRefreshToken,
  };
}

function extractID(res, label) {
  const body = parseJSON(res);
  const id = body && body.data && body.data.id ? body.data.id : "";
  if (!id) {
    fail(`${label} response missing id: ${truncate(res.body, 500)}`);
  }

  return id;
}

function extractData(res, label) {
  const body = parseJSON(res);
  if (!body || !body.data) {
    fail(`${label} response missing data: ${truncate(res.body, 500)}`);
  }

  return body.data;
}

function findBySlug(items, slug) {
  for (let i = 0; i < items.length; i += 1) {
    if (items[i] && items[i].slug === slug) {
      return items[i];
    }
  }

  return null;
}

function getRequiredDiscoveryValues(token) {
  const categoriesRes = get(
    "/api/v1/discovery/categories",
    "full_list_discovery_categories",
    token,
  );
  assertStatus(categoriesRes, 200, "list discovery categories");
  const categoriesData = extractData(categoriesRes, "list discovery categories");
  const categories = categoriesData.categories || [];
  const foodCategory = findBySlug(categories, "food");
  if (!foodCategory) {
    fail(`food discovery category is missing: ${truncate(categoriesRes.body, 500)}`);
  }

  const subcategoriesRes = get(
    "/api/v1/discovery/subcategories",
    "full_list_discovery_subcategories",
    token,
  );
  assertStatus(subcategoriesRes, 200, "list discovery subcategories");
  const subcategoriesData = extractData(
    subcategoriesRes,
    "list discovery subcategories",
  );
  const subcategories = subcategoriesData.subcategories || [];
  const mealSubcategory = findBySlug(subcategories, "meal");
  if (!mealSubcategory) {
    fail(
      `meal discovery subcategory is missing: ${truncate(subcategoriesRes.body, 500)}`,
    );
  }
  if (mealSubcategory.category_id !== foodCategory.id) {
    fail("meal discovery subcategory does not belong to food category");
  }

  const hubsRes = get("/api/v1/discovery/hubs", "full_list_discovery_hubs", token);
  assertStatus(hubsRes, 200, "list discovery hubs");
  const hubsData = extractData(hubsRes, "list discovery hubs");
  if (!hubsData.hubs) {
    fail(`discovery hubs response missing hubs: ${truncate(hubsRes.body, 500)}`);
  }

  return {
    categoryID: foodCategory.id,
    subcategoryID: mealSubcategory.id,
  };
}

function assertSearchFindsMerchant(path, requestName, token, merchantID) {
  const res = get(path, requestName, token);
  assertStatus(res, 200, requestName);

  const data = extractData(res, requestName);
  const merchants = data.merchants || [];
  let found = null;
  for (let i = 0; i < merchants.length; i += 1) {
    if (merchants[i] && merchants[i].merchant_id === merchantID) {
      found = merchants[i];
      break;
    }
  }
  if (!found) {
    fail(`${requestName} did not return merchant ${merchantID}: ${truncate(res.body, 500)}`);
  }

  return found;
}

export function setup() {
  const healthRes = get("/health", "full_setup_health");
  assertStatus(healthRes, 200, "setup health");

  const publicTestRes = get("/test/public", "full_setup_test_public");
  assertStatus(publicTestRes, 200, "setup public test route");

  const registerRes = post(
    "/auth/register/merchant",
    {
      name: SETUP_MERCHANT_NAME,
      email: SETUP_MERCHANT_EMAIL,
      password: DEFAULT_PASSWORD,
      store_name: SETUP_STORE_NAME,
    },
    "full_setup_register_merchant",
  );
  assertStatuses(registerRes, [201, 409], "setup register merchant");

  const merchantLogin = login(
    SETUP_MERCHANT_EMAIL,
    DEFAULT_PASSWORD,
    "full_setup_login_merchant",
  );
  logout(merchantLogin.refreshToken, "full_setup_logout_merchant");

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
    "/auth/register/user",
    {
      name: `k6-user-${userSuffix}`,
      email: userEmail,
      password: userPassword,
    },
    "full_register_user",
  );
  assertStatuses(registerUserRes, [201, 409], "register user");

  let userLogin = null;
  let merchantLogin = null;

  try {
    userLogin = login(userEmail, userPassword, "full_login_user");
    const refreshedUser = refresh(userLogin.refreshToken, "full_refresh_user");
    userLogin.token = refreshedUser.token;
    userLogin.refreshToken = refreshedUser.refreshToken;

    merchantLogin = login(
      setupData.merchantEmail,
      setupData.merchantPassword,
      "full_login_merchant",
    );

    const userProfileRes = get(
      "/api/v1/user/profile",
      "full_get_profile",
      userLogin.token,
    );
    assertStatus(userProfileRes, 200, "get user profile");
    check(parseJSON(userProfileRes), {
      "profile has id": (body) => body && body.data && body.data.id,
      "profile has email": (body) => body && body.data && body.data.email,
    });

    const authTestRes = get(
      "/test/auth",
      "full_get_test_auth",
      userLogin.token,
    );
    assertStatus(authTestRes, 200, "authenticated test route");

    const merchantProfileRes = get(
      "/api/v1/user/profile",
      "full_get_merchant_profile",
      merchantLogin.token,
    );
    assertStatus(merchantProfileRes, 200, "get merchant profile");

    const merchantVerificationRes = post(
      "/api/v1/merchant/verification",
      {
        business_license: `K6-LICENSE-${userSuffix}`,
      },
      "full_submit_merchant_verification",
      merchantLogin.token,
    );
    assertStatus(merchantVerificationRes, 200, "submit merchant verification");

    const userLocationRes = post(
      "/api/v1/locations/user",
      {
        label: `home-${userSuffix}`,
        full_address: "No. 1, Test Road, Taipei",
        latitude: 25.033,
        longitude: 121.5654,
        is_primary: true,
        is_active: true,
      },
      "full_create_user_location",
      userLogin.token,
    );
    assertStatus(userLocationRes, 201, "create user location");
    const userLocationBody = parseJSON(userLocationRes);
    const userLocationID =
      userLocationBody && userLocationBody.data && userLocationBody.data.id
        ? userLocationBody.data.id
        : "";
    if (!userLocationID) {
      fail(
        `create user location response missing id: ${truncate(userLocationRes.body, 500)}`,
      );
    }

    const updateUserLocationRes = put(
      `/api/v1/locations/user/${userLocationID}`,
      {
        label: `home-updated-${userSuffix}`,
        is_active: true,
      },
      "full_update_user_location",
      userLogin.token,
    );
    assertStatus(updateUserLocationRes, 200, "update user location");

    const listUserLocationsRes = get(
      "/api/v1/locations/user",
      "full_list_user_locations",
      userLogin.token,
    );
    assertStatus(listUserLocationsRes, 200, "list user locations");

    const registerDeviceRes = post(
      "/api/v1/devices",
      {
        fcm_token: `fcm-token-${userSuffix}`,
        device_id: `device-${userSuffix}`,
        platform: "android",
      },
      "full_register_device",
      userLogin.token,
    );
    assertStatus(registerDeviceRes, 201, "register device");
    const registerDeviceBody = parseJSON(registerDeviceRes);
    const deviceID =
      registerDeviceBody &&
      registerDeviceBody.data &&
      registerDeviceBody.data.id
        ? registerDeviceBody.data.id
        : "";
    if (!deviceID) {
      fail(
        `register device response missing id: ${truncate(registerDeviceRes.body, 500)}`,
      );
    }

    const getDevicesRes = get(
      "/api/v1/devices",
      "full_get_devices",
      userLogin.token,
    );
    assertStatus(getDevicesRes, 200, "get devices");

    const deviceHealthRes = get(
      "/api/v1/devices/health",
      "full_get_device_health",
      userLogin.token,
    );
    assertStatus(deviceHealthRes, 200, "get device health");

    const updateDeviceTokenRes = put(
      `/api/v1/devices/${deviceID}/token`,
      {
        fcm_token: `fcm-token-updated-${userSuffix}`,
      },
      "full_update_device_token",
      userLogin.token,
    );
    assertStatus(updateDeviceTokenRes, 200, "update device token");

    const subscribeRes = post(
      "/api/v1/subscriptions",
      {
        merchant_id: setupData.merchantID,
        device_info: {
          fcm_token: `subscription-fcm-token-${userSuffix}`,
          device_id: `subscription-device-${userSuffix}`,
          platform: "ios",
        },
      },
      "full_subscribe",
      userLogin.token,
    );
    assertStatus(subscribeRes, 201, "subscribe merchant");

    const listSubscriptionsRes = get(
      "/api/v1/subscriptions",
      "full_list_subscriptions",
      userLogin.token,
    );
    assertStatus(listSubscriptionsRes, 200, "list subscriptions");

    const merchantLocationRes = post(
      "/api/v1/locations/merchant",
      {
        label: `merchant-location-${userSuffix}`,
        full_address: "No. 100, Merchant Street, Taipei",
        latitude: 25.0478,
        longitude: 121.5319,
        is_primary: true,
        is_active: true,
      },
      "full_create_merchant_location",
      merchantLogin.token,
    );
    assertStatus(merchantLocationRes, 201, "create merchant location");
    const merchantLocationID = extractID(
      merchantLocationRes,
      "create merchant location",
    );

    const listMerchantLocationsRes = get(
      "/api/v1/locations/merchant",
      "full_list_merchant_locations",
      merchantLogin.token,
    );
    assertStatus(listMerchantLocationsRes, 200, "list merchant locations");

    const updateMerchantLocationRes = put(
      `/api/v1/locations/merchant/${merchantLocationID}`,
      {
        label: `merchant-location-updated-${userSuffix}`,
        is_active: true,
      },
      "full_update_merchant_location",
      merchantLogin.token,
    );
    assertStatus(updateMerchantLocationRes, 200, "update merchant location");

    const discoveryValues = getRequiredDiscoveryValues(userLogin.token);

    const getDiscoveryProfileRes = get(
      "/api/v1/merchant/discovery-profile",
      "full_get_merchant_discovery_profile",
      merchantLogin.token,
    );
    assertStatus(
      getDiscoveryProfileRes,
      200,
      "get merchant discovery profile",
    );

    const updateDiscoveryProfileRes = patch(
      "/api/v1/merchant/discovery-profile",
      {
        discovery_category_id: discoveryValues.categoryID,
        discovery_subcategory_id: discoveryValues.subcategoryID,
        active_hub_id: null,
        is_public: true,
      },
      "full_update_merchant_discovery_profile_public",
      merchantLogin.token,
    );
    assertStatus(
      updateDiscoveryProfileRes,
      200,
      "update merchant discovery profile public",
    );
    check(parseJSON(updateDiscoveryProfileRes), {
      "merchant discovery profile is public": (body) =>
        body && body.data && body.data.is_public === true,
      "merchant discovery profile is eligible": (body) =>
        body &&
        body.data &&
        body.data.is_verified === true &&
        body.data.has_active_primary_location === true,
    });

    const categorySearchMerchant = assertSearchFindsMerchant(
      "/api/v1/merchants?category_slug=food&page=1&page_size=20",
      "full_search_merchants_by_category",
      userLogin.token,
      setupData.merchantID,
    );
    check(categorySearchMerchant, {
      "category search has discovery category": (merchant) =>
        merchant.discovery_category &&
        merchant.discovery_category.slug === "food",
      "category search has primary location": (merchant) =>
        merchant.primary_location && merchant.primary_location.id,
    });

    const nearbySearchMerchant = assertSearchFindsMerchant(
      "/api/v1/merchants?category_slug=food&subcategory_slug=meal&latitude=25.0478&longitude=121.5319&radius_meters=3000&page=1&page_size=20",
      "full_search_merchants_nearby",
      userLogin.token,
      setupData.merchantID,
    );
    check(nearbySearchMerchant, {
      "nearby search includes distance": (merchant) =>
        typeof merchant.distance_meters === "number",
    });

    const createMenuRes = post(
      "/api/v1/menus/merchant",
      {
        name: `k6-menu-${userSuffix}`,
        description: "k6 full flow menu item",
        category: "main",
        price: 120,
        currency: "TWD",
        prep_minutes: 10,
        is_available: true,
        is_popular: true,
      },
      "full_create_menu_item",
      merchantLogin.token,
    );
    assertStatus(createMenuRes, 201, "create menu item");
    const menuItemID = extractID(createMenuRes, "create menu item");

    const listMerchantMenuRes = get(
      "/api/v1/menus/merchant",
      "full_list_merchant_menu",
      merchantLogin.token,
    );
    assertStatus(listMerchantMenuRes, 200, "list merchant menu");

    const updateMenuStatusRes = patch(
      `/api/v1/menus/merchant/${menuItemID}/status`,
      {
        is_available: true,
      },
      "full_update_menu_status",
      merchantLogin.token,
    );
    assertStatus(updateMenuStatusRes, 200, "update menu status");

    const updateMenuRes = put(
      `/api/v1/menus/merchant/${menuItemID}`,
      {
        name: `k6-menu-updated-${userSuffix}`,
        description: "k6 full flow updated menu item",
        category: "main",
        price: 130,
        currency: "TWD",
        prep_minutes: 12,
        is_available: true,
        is_popular: false,
      },
      "full_update_menu_item",
      merchantLogin.token,
    );
    assertStatus(updateMenuRes, 200, "update menu item");

    const reorderMenuRes = patch(
      "/api/v1/menus/merchant/reorder",
      {
        item_ids: [menuItemID],
      },
      "full_reorder_menu",
      merchantLogin.token,
    );
    assertStatus(reorderMenuRes, 200, "reorder menu");

    const publicMenuRes = get(
      `/api/v1/merchants/${setupData.merchantID}/menu`,
      "full_get_public_menu",
      userLogin.token,
    );
    assertStatus(publicMenuRes, 200, "get public menu");

    const merchantQRRes = get(
      "/api/v1/merchant/qr",
      "full_get_merchant_qr",
      merchantLogin.token,
    );
    assertStatus(merchantQRRes, 200, "get merchant qr");

    const publishNotificationRes = post(
      "/api/v1/notifications",
      {
        location_data: {
          location_name: `spot-${userSuffix}`,
          full_address: "No. 100, Merchant Street, Taipei",
          latitude: 25.0478,
          longitude: 121.5319,
        },
        hint_message: "k6 full test",
      },
      "full_publish_notification",
      merchantLogin.token,
    );
    assertStatus(publishNotificationRes, 201, "publish notification");

    const historyRes = get(
      "/api/v1/notifications?limit=10&offset=0",
      "full_get_notification_history",
      merchantLogin.token,
    );
    assertStatus(historyRes, 200, "get notification history");

    const privateDiscoveryProfileRes = patch(
      "/api/v1/merchant/discovery-profile",
      {
        is_public: false,
      },
      "full_update_merchant_discovery_profile_private",
      merchantLogin.token,
    );
    assertStatus(
      privateDiscoveryProfileRes,
      200,
      "update merchant discovery profile private",
    );

    const deleteMenuRes = del(
      `/api/v1/menus/merchant/${menuItemID}`,
      "full_delete_menu_item",
      merchantLogin.token,
    );
    assertStatus(deleteMenuRes, 200, "delete menu item");

    const deleteMerchantLocationRes = del(
      `/api/v1/locations/merchant/${merchantLocationID}`,
      "full_delete_merchant_location",
      merchantLogin.token,
    );
    assertStatus(deleteMerchantLocationRes, 200, "delete merchant location");

    const unsubscribeRes = del(
      `/api/v1/subscriptions/${setupData.merchantID}`,
      "full_unsubscribe",
      userLogin.token,
    );
    assertStatus(unsubscribeRes, 200, "unsubscribe merchant");

    const deleteUserLocationRes = del(
      `/api/v1/locations/user/${userLocationID}`,
      "full_delete_user_location",
      userLogin.token,
    );
    assertStatus(deleteUserLocationRes, 200, "delete user location");

    const deactivateDeviceRes = del(
      `/api/v1/devices/${deviceID}`,
      "full_deactivate_device",
      userLogin.token,
    );
    assertStatus(deactivateDeviceRes, 200, "deactivate device");

    sleep(FULL_SLEEP_SECONDS);
  } finally {
    if (userLogin) {
      tryLogout(userLogin.refreshToken, "full_finally_logout_user");
    }

    if (merchantLogin) {
      tryLogout(merchantLogin.refreshToken, "full_finally_logout_merchant");
    }
  }
}
