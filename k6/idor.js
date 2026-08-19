import http from "k6/http";
import { check, fail } from "k6";

const BASE_URL = (__ENV.BASE_URL || "http://localhost:4433").replace(/\/$/, "");
const RUN_ID = __ENV.RUN_ID || String(Date.now());
const SAFE_RUN_ID = RUN_ID.replace(/[^a-zA-Z0-9_-]/g, "-");
const PASSWORD = __ENV.K6_TEST_PASSWORD || "K6pass!1234";
const API_PREFIX = "/api/v1";

export const options = {
  scenarios: {
    idor: {
      executor: "shared-iterations",
      vus: 1,
      iterations: 1,
      maxDuration: __ENV.IDOR_MAX_DURATION || "5m",
    },
  },
  thresholds: {
    checks: ["rate==1"],
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

function request(method, path, token, payload, name) {
  const headers = {};
  if (payload !== undefined) {
    headers["Content-Type"] = "application/json";
  }
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  return http.request(
    method,
    `${BASE_URL}${path}`,
    payload === undefined ? null : JSON.stringify(payload),
    {
      headers: headers,
      tags: { name: name },
    },
  );
}

function assertStatus(res, expectedStatus, label) {
  const ok = check(res, {
    [`${label} status is ${expectedStatus}`]: (response) =>
      response.status === expectedStatus,
  });

  if (!ok) {
    fail(
      `${label} failed with status=${res.status} body=${truncate(res.body, 500)}`,
    );
  }
}

function assertRegistrationStatus(res, label) {
  const ok = check(res, {
    [`${label} status is 201 or 409`]: (response) =>
      response.status === 201 || response.status === 409,
  });

  if (!ok) {
    fail(
      `${label} failed with status=${res.status} body=${truncate(res.body, 500)}`,
    );
  }
}

function assertDenied(res, label, expectedCode) {
  assertStatus(res, 403, label);

  const body = parseJSON(res);
  const code = body && body.error ? body.error.code : "";
  const ok = check(body, {
    [`${label} returns ${expectedCode}`]: () => code === expectedCode,
  });

  if (!ok) {
    fail(`${label} returned unexpected error code=${code}`);
  }
}

function extractData(res, label) {
  const body = parseJSON(res);
  if (!body || body.data === undefined || body.data === null) {
    fail(`${label} response missing data: ${truncate(res.body, 500)}`);
  }

  return body.data;
}

function extractID(res, label) {
  const data = extractData(res, label);
  if (!data.id) {
    fail(`${label} response missing id: ${truncate(res.body, 500)}`);
  }

  return data.id;
}

function findByID(items, id, label) {
  for (let index = 0; index < items.length; index += 1) {
    if (items[index] && items[index].id === id) {
      return items[index];
    }
  }

  fail(`${label} did not contain resource ${id}`);
}

function findMerchantByID(items, merchantID, label) {
  for (let index = 0; index < items.length; index += 1) {
    if (items[index] && items[index].merchant_id === merchantID) {
      return items[index];
    }
  }

  fail(`${label} did not contain merchant ${merchantID}`);
}

function assertPublicDataEqual(firstRes, secondRes, label) {
  const firstData = extractData(firstRes, `${label} owner response`);
  const secondData = extractData(secondRes, `${label} other response`);
  const firstJSON = JSON.stringify(firstData);
  const secondJSON = JSON.stringify(secondData);
  const ok = check(
    { firstData: firstData, secondData: secondData },
    {
      [`${label} data is identical for both auth users`]: () =>
        firstJSON === secondJSON,
    },
  );

  if (!ok) {
    fail(
      `${label} data differed: owner=${truncate(firstJSON, 500)} other=${truncate(
        secondJSON,
        500,
      )}`,
    );
  }

  return firstData;
}

function login(account, label) {
  const res = request(
    "POST",
    "/auth/login",
    "",
    { email: account.email, password: account.password },
    label,
  );
  assertStatus(res, 200, `${label} login`);

  const data = extractData(res, `${label} login`);
  if (!data.access_token || !data.refresh_token || !data.user || !data.user.id) {
    fail(`${label} login response missing credentials: ${truncate(res.body, 500)}`);
  }

  return {
    email: account.email,
    password: account.password,
    token: data.access_token,
    refreshToken: data.refresh_token,
    userID: data.user.id,
  };
}

function registerAndLogin(role, label) {
  const email = `k6-idor-${SAFE_RUN_ID}-${label}@example.com`;
  const account = {
    email: email,
    password: PASSWORD,
  };

  const payload = {
    name: `k6-idor-${label}-${SAFE_RUN_ID}`,
    email: email,
    password: PASSWORD,
  };
  if (role === "merchant") {
    payload.store_name = merchantStoreName(label);
  }

  const res = request(
    "POST",
    `/auth/register/${role}`,
    "",
    payload,
    `idor_register_${label}`,
  );
  assertRegistrationStatus(res, `idor register ${label}`);

  const authenticated = login(account, `idor_${label}`);
  if (role === "merchant") {
    authenticated.storeName = payload.store_name;
  }

  return authenticated;
}

function merchantStoreName(label) {
  return `K6 IDOR Store ${label} ${SAFE_RUN_ID}`;
}

function registerUserRoleAndLogin(merchantAccount, label) {
  const res = request(
    "POST",
    "/auth/register/user",
    "",
    {
      name: `k6-idor-${label}-${SAFE_RUN_ID}`,
      email: merchantAccount.email,
      password: merchantAccount.password,
    },
    `idor_register_${label}`,
  );
  assertRegistrationStatus(res, `idor register ${label}`);

  const authenticated = login(merchantAccount, `idor_${label}`);
  const ok = check(authenticated, {
    [`${label} keeps the merchant account user ID`]: (account) =>
      account.userID === merchantAccount.userID,
  });
  if (!ok) {
    fail(`${label} returned a different user ID from the merchant account`);
  }

  return authenticated;
}

function getDiscoveryValues(token) {
  const categoriesRes = request(
    "GET",
    `${API_PREFIX}/discovery/categories`,
    token,
    undefined,
    "idor_list_discovery_categories",
  );
  assertStatus(categoriesRes, 200, "idor list discovery categories");

  const categoriesData = extractData(
    categoriesRes,
    "idor list discovery categories",
  );
  const categories = categoriesData.categories || [];
  let category = null;
  for (let index = 0; index < categories.length; index += 1) {
    if (categories[index] && categories[index].id) {
      category = categories[index];
      break;
    }
  }
  if (!category) {
    fail(`idor discovery response has no usable category: ${truncate(categoriesRes.body, 500)}`);
  }

  const res = request(
    "GET",
    `${API_PREFIX}/discovery/subcategories`,
    token,
    undefined,
    "idor_list_discovery_subcategories",
  );
  assertStatus(res, 200, "idor list discovery subcategories");

  const data = extractData(res, "idor list discovery subcategories");
  const subcategories = data.subcategories || [];
  for (let index = 0; index < subcategories.length; index += 1) {
    if (
      subcategories[index] &&
      subcategories[index].id &&
      subcategories[index].category_id === category.id
    ) {
      return {
        categoryID: category.id,
        subcategoryID: subcategories[index].id,
      };
    }
  }

  fail(
    `idor discovery response has no usable subcategory for category ${category.id}: ${truncate(
      res.body,
      500,
    )}`,
  );
}

function createLocation(token, path, label, requestName) {
  const res = request(
    "POST",
    path,
    token,
    {
      label: label,
      full_address: "No. 1, IDOR Test Road, Taipei",
      latitude: 25.033,
      longitude: 121.5654,
      is_primary: true,
      is_active: true,
    },
    requestName,
  );
  assertStatus(res, 201, requestName);

  return extractID(res, requestName);
}

function createDevice(token) {
  const requestName = "idor_create_device";
  const res = request(
    "POST",
    `${API_PREFIX}/devices`,
    token,
    {
      fcm_token: `idor-original-token-${SAFE_RUN_ID}`,
      device_id: `idor-device-${SAFE_RUN_ID}`,
      platform: "android",
    },
    requestName,
  );
  assertStatus(res, 201, requestName);

  return extractID(res, requestName);
}

function createMenuItem(token, categoryID) {
  const requestName = "idor_create_menu_item";
  const res = request(
    "POST",
    `${API_PREFIX}/menus/merchant`,
    token,
    {
      name: `idor-original-menu-${SAFE_RUN_ID}`,
      description: "IDOR ownership test item",
      category_id: categoryID,
      price: 120,
      currency: "TWD",
      prep_minutes: 10,
      is_available: true,
      is_popular: false,
    },
    requestName,
  );
  assertStatus(res, 201, requestName);

  return extractID(res, requestName);
}

function assertLocationUnchanged(token, path, locationID, expectedLabel, label) {
  const res = request("GET", path, token, undefined, `${label}_list`);
  assertStatus(res, 200, `${label} list`);

  const location = findByID(extractData(res, `${label} list`), locationID, label);
  const ok = check(location, {
    [`${label} remains unchanged`]: (item) => item.label === expectedLabel,
  });
  if (!ok) {
    fail(`${label} was modified by a different owner`);
  }
}

function assertDeviceUnchanged(token, deviceID) {
  const label = "idor device";
  const res = request("GET", `${API_PREFIX}/devices`, token, undefined, "idor_list_devices");
  assertStatus(res, 200, "idor list devices");

  const device = findByID(extractData(res, "idor list devices"), deviceID, label);
  const ok = check(device, {
    "idor device token remains unchanged": (item) =>
      item.fcm_token === `idor-original-token-${SAFE_RUN_ID}`,
    "idor device remains active": (item) => item.is_active === true,
  });
  if (!ok) {
    fail("idor device was modified by a different owner");
  }
}

function assertMenuUnchanged(token, menuItemID) {
  const label = "idor menu item";
  const res = request(
    "GET",
    `${API_PREFIX}/menus/merchant`,
    token,
    undefined,
    "idor_list_merchant_menu",
  );
  assertStatus(res, 200, "idor list merchant menu");

  const data = extractData(res, "idor list merchant menu");
  const item = findByID(data.items || [], menuItemID, label);
  const ok = check(item, {
    "idor menu name remains unchanged": (menuItem) =>
      menuItem.name === `idor-original-menu-${SAFE_RUN_ID}`,
    "idor menu availability remains unchanged": (menuItem) =>
      menuItem.is_available === true,
  });
  if (!ok) {
    fail("idor menu item was modified by a different owner");
  }
}

function tryLogout(account, label) {
  if (!account || !account.refreshToken) {
    return;
  }

  const res = request(
    "POST",
    "/auth/logout",
    "",
    { refresh_token: account.refreshToken },
    label,
  );
  if (res.status !== 200) {
    console.error(`${label} failed with status=${res.status}`);
  }
}

export function setup() {
  const healthRes = request("GET", "/health", "", undefined, "idor_setup_health");
  assertStatus(healthRes, 200, "idor setup health");

  const merchantA = registerAndLogin("merchant", "merchant-a");
  const merchantAUser = registerUserRoleAndLogin(merchantA, "merchant-a-user");
  const userA = registerAndLogin("user", "user-a");
  const userB = registerAndLogin("user", "user-b");
  const merchantB = registerAndLogin("merchant", "merchant-b");
  const identityCheck = check(
    { owner: merchantAUser, other: userA },
    {
      "idor read parity uses a different authenticated user": (value) =>
        value.owner.userID !== value.other.userID,
    },
  );
  if (!identityCheck) {
    fail("idor read parity owner and other users must be different accounts");
  }

  return {
    userA: userA,
    userB: userB,
    merchantA: merchantA,
    merchantAUser: merchantAUser,
    merchantB: merchantB,
  };
}

export default function (setupData) {
  const userA = setupData.userA;
  const userB = setupData.userB;
  const merchantA = setupData.merchantA;
  const merchantAUser = setupData.merchantAUser;
  const merchantB = setupData.merchantB;

  try {
    const discoveryValues = getDiscoveryValues(userA.token);
    const userLocationID = createLocation(
      userA.token,
      `${API_PREFIX}/locations/user`,
      `idor-user-location-${SAFE_RUN_ID}`,
      "idor_create_user_location",
    );
    const deviceID = createDevice(userA.token);
    const merchantLocationID = createLocation(
      merchantA.token,
      `${API_PREFIX}/locations/merchant`,
      `idor-merchant-location-${SAFE_RUN_ID}`,
      "idor_create_merchant_location",
    );
    const verificationRes = request(
      "POST",
      `${API_PREFIX}/merchant/verification`,
      merchantA.token,
      { business_license: `K6-IDOR-LICENSE-${SAFE_RUN_ID}` },
      "idor_submit_merchant_verification",
    );
    assertStatus(
      verificationRes,
      200,
      "idor submit merchant verification",
    );

    const publishProfileRes = request(
      "PATCH",
      `${API_PREFIX}/merchant/discovery-profile`,
      merchantA.token,
      {
        discovery_category_id: discoveryValues.categoryID,
        discovery_subcategory_id: discoveryValues.subcategoryID,
        active_hub_id: null,
        is_public: true,
      },
      "idor_publish_merchant_discovery_profile",
    );
    assertStatus(
      publishProfileRes,
      200,
      "idor publish merchant discovery profile",
    );
    const publishProfileData = extractData(
      publishProfileRes,
      "idor publish merchant discovery profile",
    );
    const publishProfileCheck = check(publishProfileData, {
      "idor merchant discovery profile is public": (profile) =>
        profile.is_public === true,
    });
    if (!publishProfileCheck) {
      fail("idor merchant discovery profile was not made public");
    }

    const menuItemID = createMenuItem(
      merchantA.token,
      discoveryValues.subcategoryID,
    );

    const searchPath = `${API_PREFIX}/merchants?keyword=${encodeURIComponent(
      merchantA.storeName,
    )}&page=1&page_size=20`;
    const ownerSearchRes = request(
      "GET",
      searchPath,
      merchantAUser.token,
      undefined,
      "idor_owner_search_public_merchants",
    );
    const otherSearchRes = request(
      "GET",
      searchPath,
      userA.token,
      undefined,
      "idor_other_search_public_merchants",
    );
    assertStatus(ownerSearchRes, 200, "idor owner search public merchants");
    assertStatus(otherSearchRes, 200, "idor other search public merchants");

    const ownerSearchData = assertPublicDataEqual(
      ownerSearchRes,
      otherSearchRes,
      "idor public merchant search",
    );
    const ownerSearchMerchant = findMerchantByID(
      ownerSearchData.merchants || [],
      merchantA.userID,
      "idor owner public merchant search",
    );
    const otherSearchData = extractData(
      otherSearchRes,
      "idor other public merchant search",
    );
    const otherSearchMerchant = findMerchantByID(
      otherSearchData.merchants || [],
      merchantA.userID,
      "idor other public merchant search",
    );
    const searchMerchantCheck = check(
      { owner: ownerSearchMerchant, other: otherSearchMerchant },
      {
        "idor owner search returns the expected merchant": (value) =>
          value.owner.store_name === merchantA.storeName,
        "idor other search returns the expected merchant": (value) =>
          value.other.store_name === merchantA.storeName,
      },
    );
    if (!searchMerchantCheck) {
      fail("idor public merchant search returned unexpected merchant data");
    }

    const publicMenuPath = `${API_PREFIX}/merchants/${merchantA.userID}/menu?page=1&page_size=20`;
    const ownerPublicMenuRes = request(
      "GET",
      publicMenuPath,
      merchantAUser.token,
      undefined,
      "idor_owner_get_public_menu",
    );
    const otherPublicMenuRes = request(
      "GET",
      publicMenuPath,
      userA.token,
      undefined,
      "idor_other_get_public_menu",
    );
    assertStatus(ownerPublicMenuRes, 200, "idor owner get public menu");
    assertStatus(otherPublicMenuRes, 200, "idor other get public menu");

    const ownerPublicMenuData = assertPublicDataEqual(
      ownerPublicMenuRes,
      otherPublicMenuRes,
      "idor public merchant menu",
    );
    const publicMenuItem = findByID(
      ownerPublicMenuData.items || [],
      menuItemID,
      "idor public merchant menu",
    );
    const publicMenuCheck = check(publicMenuItem, {
      "idor public menu contains the created item": (item) =>
        item.name === `idor-original-menu-${SAFE_RUN_ID}`,
      "idor public menu item is available": (item) =>
        item.is_available === true,
    });
    if (!publicMenuCheck) {
      fail("idor public merchant menu did not contain the expected item");
    }

    assertDenied(
      request(
        "PUT",
        `${API_PREFIX}/locations/user/${userLocationID}`,
        userB.token,
        { label: `idor-user-attacker-${SAFE_RUN_ID}` },
        "idor_user_location_update_other_owner",
      ),
      "idor user location update by other owner",
      "ADDRESS_OWNERSHIP_VIOLATION",
    );
    assertDenied(
      request(
        "DELETE",
        `${API_PREFIX}/locations/user/${userLocationID}`,
        userB.token,
        undefined,
        "idor_user_location_delete_other_owner",
      ),
      "idor user location delete by other owner",
      "ADDRESS_OWNERSHIP_VIOLATION",
    );
    assertDenied(
      request(
        "PUT",
        `${API_PREFIX}/devices/${deviceID}/token`,
        userB.token,
        { fcm_token: `idor-attacker-token-${SAFE_RUN_ID}` },
        "idor_device_update_other_owner",
      ),
      "idor device update by other owner",
      "DEVICE_OWNERSHIP_VIOLATION",
    );
    assertDenied(
      request(
        "DELETE",
        `${API_PREFIX}/devices/${deviceID}`,
        userB.token,
        undefined,
        "idor_device_deactivate_other_owner",
      ),
      "idor device deactivate by other owner",
      "DEVICE_OWNERSHIP_VIOLATION",
    );

    assertDenied(
      request(
        "PUT",
        `${API_PREFIX}/locations/merchant/${merchantLocationID}`,
        merchantB.token,
        { label: `idor-merchant-attacker-${SAFE_RUN_ID}` },
        "idor_merchant_location_update_other_owner",
      ),
      "idor merchant location update by other owner",
      "ADDRESS_OWNERSHIP_VIOLATION",
    );
    assertDenied(
      request(
        "DELETE",
        `${API_PREFIX}/locations/merchant/${merchantLocationID}`,
        merchantB.token,
        undefined,
        "idor_merchant_location_delete_other_owner",
      ),
      "idor merchant location delete by other owner",
      "ADDRESS_OWNERSHIP_VIOLATION",
    );
    assertDenied(
      request(
        "PUT",
        `${API_PREFIX}/menus/merchant/${menuItemID}`,
        merchantB.token,
        {
          name: `idor-attacker-menu-${SAFE_RUN_ID}`,
          description: "unauthorized update",
          category_id: discoveryValues.subcategoryID,
          price: 999,
          currency: "TWD",
          prep_minutes: 20,
          is_available: false,
          is_popular: false,
        },
        "idor_menu_update_other_owner",
      ),
      "idor menu update by other owner",
      "FORBIDDEN_RESOURCE_OWNER",
    );
    assertDenied(
      request(
        "PATCH",
        `${API_PREFIX}/menus/merchant/${menuItemID}/status`,
        merchantB.token,
        { is_available: false },
        "idor_menu_status_other_owner",
      ),
      "idor menu status update by other owner",
      "FORBIDDEN_RESOURCE_OWNER",
    );
    assertDenied(
      request(
        "DELETE",
        `${API_PREFIX}/menus/merchant/${menuItemID}`,
        merchantB.token,
        undefined,
        "idor_menu_delete_other_owner",
      ),
      "idor menu delete by other owner",
      "FORBIDDEN_RESOURCE_OWNER",
    );

    assertLocationUnchanged(
      userA.token,
      `${API_PREFIX}/locations/user`,
      userLocationID,
      `idor-user-location-${SAFE_RUN_ID}`,
      "idor user location",
    );
    assertDeviceUnchanged(userA.token, deviceID);
    assertLocationUnchanged(
      merchantA.token,
      `${API_PREFIX}/locations/merchant`,
      merchantLocationID,
      `idor-merchant-location-${SAFE_RUN_ID}`,
      "idor merchant location",
    );
    assertMenuUnchanged(merchantA.token, menuItemID);
  } finally {
    tryLogout(userA, "idor_logout_user_a");
    tryLogout(userB, "idor_logout_user_b");
    tryLogout(merchantAUser, "idor_logout_merchant_a_user");
    tryLogout(merchantA, "idor_logout_merchant_a");
    tryLogout(merchantB, "idor_logout_merchant_b");
  }
}
