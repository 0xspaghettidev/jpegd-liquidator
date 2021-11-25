// SPDX-License-Identifier: GPL-3.0
pragma solidity ^0.8.0;

interface INFTVault {

    struct Rate {
        uint128 numerator;
        uint128 denominator;
    }

    struct VaultSettings {
        Rate debtInterestApr;
        Rate creditLimitRate;
        Rate liquidationLimitRate;
        Rate valueIncreaseLockRate;
        Rate organizationFeeRate;
        Rate insurancePurchaseRate;
        Rate insuranceLiquidationPenaltyRate;
        uint256 insuraceRepurchaseTimeLimit;
        uint256 borrowAmountCap;
    }

    struct PositionPreview {
        address owner;
        uint256 nftIndex;
        bytes32 nftType;
        uint256 nftValueUSD;
        VaultSettings vaultSettings;
        uint256 creditLimit;
        uint256 debtPrincipal;
        uint256 debtInterest;
        BorrowType borrowType;
        bool liquidatable;
        uint256 liquidatedAt;
        address liquidator;
    }

    enum BorrowType {
        NOT_CONFIRMED,
        NON_INSURANCE,
        USE_INSURANCE
    }

    function showPosition(uint256 _nftIndex)
        external
        view
        returns (PositionPreview memory preview);

    function liquidate(uint256 _nftIndex) external;

    function claimExpiredInsuranceNFT(uint256 _nftIndex) external;

    function openPositionsIndexes() external returns (uint256[] memory);
}