// SPDX-License-Identifier: GPL-3.0
pragma solidity ^0.8.0;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/utils/Address.sol";

interface INFTVault {
    event PositionOpened(address owner, uint256 index);
    event PositionClosed(address owner, uint256 index);
    event Liquidated(address liquidator, address owner, uint256 index);
    event Repurchased(address owner, uint256 index);
    event InsuranceExpired(address owner, uint256 index);

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
}

interface ICryptoPunks {
    function transferPunk(address to, uint256 punkIndex) external;
}

contract PunkLiquidator {
    using Address for address;

    INFTVault nftVault;
    ICryptoPunks cryptoPunks;
    IERC20 pusd;
    address dao;

    constructor(
        INFTVault _nftVault,
        ICryptoPunks _cryptoPunks,
        IERC20 _pusd,
        address _dao
    ) {
        nftVault = _nftVault;
        cryptoPunks = _cryptoPunks;
        pusd = _pusd;
        dao = _dao;
    }

    function liquidate(uint256[] memory _toLiquidate) external {
        uint256 balance = pusd.balanceOf(address(this));
        for (uint256 i = 0; i < _toLiquidate.length; i++) {
            uint256 nftIndex = _toLiquidate[i];
            INFTVault.PositionPreview memory position = nftVault.showPosition(
                nftIndex
            );
            if (!position.liquidatable) continue;

            uint256 totalDebt = position.debtPrincipal + position.debtInterest;
            if (totalDebt > balance) continue;

            pusd.approve(address(nftVault), totalDebt);
            nftVault.liquidate(nftIndex);

            if (position.borrowType == INFTVault.BorrowType.NON_INSURANCE) {
                cryptoPunks.transferPunk(dao, nftIndex);
            }
        }
    }

    function claimExpiredInsuranceNFT(uint256[] memory _toClaim) external {
        for (uint256 i = 0; i < _toClaim.length; i++) {
            uint256 nftIndex = _toClaim[i];

            INFTVault.PositionPreview memory position = nftVault.showPosition(
                nftIndex
            );

            uint256 elapsed = block.timestamp - position.liquidatedAt;

            if (elapsed < position.vaultSettings.insuraceRepurchaseTimeLimit)
                continue;

            nftVault.claimExpiredInsuranceNFT(nftIndex);
            cryptoPunks.transferPunk(dao, nftIndex);
        }
    }

    function doCalls(address[] memory targets, bytes[] memory calldatas, uint256[] memory values) external {
        require(msg.sender == dao, "unauthorized");
        
        for (uint256 i = 0; i < targets.length; i++) {
            targets[i].functionCallWithValue(calldatas[i], values[i]);
        }
    }
}
